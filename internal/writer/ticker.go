package writer

import (
	"context"
	"log/slog"
	"sync"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/rickgao/kalshi-data/internal/router"
)

// TickerWriter consumes TickerMsg from the router buffer and writes to the tickers table.
type TickerWriter struct {
	cfg    WriterConfig
	logger *slog.Logger

	// Input from Message Router
	input *router.GrowableBuffer[router.TickerMsg]

	// Database
	db *pgxpool.Pool

	// Batching
	batch       []tickerRow
	batchMu     sync.Mutex
	flushTicker *time.Ticker

	// Lifecycle
	ctx    context.Context
	cancel context.CancelFunc
	wg     sync.WaitGroup

	// Metrics
	metrics WriterMetrics
}

// NewTickerWriter creates a new TickerWriter.
func NewTickerWriter(
	cfg WriterConfig,
	input *router.GrowableBuffer[router.TickerMsg],
	db *pgxpool.Pool,
	logger *slog.Logger,
) *TickerWriter {
	if logger == nil {
		logger = slog.Default()
	}
	return &TickerWriter{
		cfg:    cfg,
		input:  input,
		db:     db,
		logger: logger,
		batch:  make([]tickerRow, 0, cfg.BatchSize),
	}
}

// Start begins consuming messages and writing to the database.
func (w *TickerWriter) Start(ctx context.Context) error {
	w.ctx, w.cancel = context.WithCancel(ctx)
	w.flushTicker = time.NewTicker(w.cfg.FlushInterval)

	// Consumer goroutine
	w.wg.Add(1)
	go w.consumeLoop()

	// Flush ticker goroutine
	w.wg.Add(1)
	go w.flushLoop()

	w.logger.Info("ticker writer started",
		"batch_size", w.cfg.BatchSize,
		"flush_interval", w.cfg.FlushInterval,
	)
	return nil
}

// Stop gracefully shuts down the writer.
func (w *TickerWriter) Stop(ctx context.Context) error {
	w.logger.Info("stopping ticker writer")

	if w.cancel != nil {
		w.cancel()
	}

	if w.flushTicker != nil {
		w.flushTicker.Stop()
	}

	// Wait for goroutines
	done := make(chan struct{})
	go func() {
		w.wg.Wait()
		close(done)
	}()

	select {
	case <-done:
		w.logger.Info("ticker writer stopped")
	case <-ctx.Done():
		w.logger.Warn("ticker writer stop timed out")
	}

	// Final flush
	w.flush()

	return nil
}

// Stats returns current metrics.
func (w *TickerWriter) Stats() WriterMetrics {
	w.batchMu.Lock()
	defer w.batchMu.Unlock()
	return w.metrics
}

// consumeLoop reads from the input buffer and accumulates batches.
func (w *TickerWriter) consumeLoop() {
	defer w.wg.Done()

	for {
		select {
		case <-w.ctx.Done():
			return
		default:
			msg, ok := w.input.TryReceive()
			if !ok {
				select {
				case <-w.ctx.Done():
					return
				case <-time.After(10 * time.Millisecond):
					continue
				}
			}

			w.handleMessage(msg)
		}
	}
}

// flushLoop periodically flushes the batch.
func (w *TickerWriter) flushLoop() {
	defer w.wg.Done()

	for {
		select {
		case <-w.ctx.Done():
			return
		case <-w.flushTicker.C:
			w.flush()
		}
	}
}

// handleMessage transforms and adds a message to the batch.
func (w *TickerWriter) handleMessage(msg router.TickerMsg) {
	row := w.transform(msg)

	w.batchMu.Lock()
	w.batch = append(w.batch, row)
	shouldFlush := len(w.batch) >= w.cfg.BatchSize
	w.batchMu.Unlock()

	if shouldFlush {
		w.flush()
	}
}

// transform converts a TickerMsg to a tickerRow.
func (w *TickerWriter) transform(msg router.TickerMsg) tickerRow {
	return tickerRow{
		ExchangeTs:         msg.ExchangeTs,
		ReceivedAt:         msg.ReceivedAt.UnixMicro(),
		Ticker:             msg.Ticker,
		YesBid:             dollarsToInternal(msg.YesBidDollars),
		YesAsk:             dollarsToInternal(msg.YesAskDollars),
		LastPrice:          dollarsToInternal(msg.PriceDollars),
		Volume:             msg.Volume,
		OpenInterest:       msg.OpenInterest,
		DollarVolume:       msg.DollarVolume,
		DollarOpenInterest: msg.DollarOpenInterest,
		SID:                msg.SID,
	}
}

// flush writes the current batch to the database.
func (w *TickerWriter) flush() {
	w.batchMu.Lock()
	if len(w.batch) == 0 {
		w.batchMu.Unlock()
		return
	}

	// Take ownership of current batch
	batch := w.batch
	w.batch = make([]tickerRow, 0, w.cfg.BatchSize)
	w.batchMu.Unlock()

	start := time.Now()

	conflicts, err := w.batchInsert(batch)
	if err != nil {
		w.logger.Error("batch insert failed", "error", err, "count", len(batch))
		w.batchMu.Lock()
		w.metrics.Errors++
		w.batchMu.Unlock()
		return
	}

	w.batchMu.Lock()
	w.metrics.Inserts += int64(len(batch) - conflicts)
	w.metrics.Conflicts += int64(conflicts)
	w.metrics.Flushes++
	w.batchMu.Unlock()

	w.logger.Debug("flushed tickers",
		"count", len(batch),
		"conflicts", conflicts,
		"duration", time.Since(start),
	)
}

// batchInsert inserts rows using pgx.Batch with ON CONFLICT DO NOTHING.
func (w *TickerWriter) batchInsert(rows []tickerRow) (conflicts int, err error) {
	batch := &pgx.Batch{}
	for _, r := range rows {
		batch.Queue(`
			INSERT INTO tickers (exchange_ts, received_at, ticker, yes_bid, yes_ask, last_price, volume, open_interest, dollar_volume, dollar_open_interest, sid)
			VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11)
			ON CONFLICT (ticker, exchange_ts) DO NOTHING
		`, r.ExchangeTs, r.ReceivedAt, r.Ticker, r.YesBid, r.YesAsk, r.LastPrice, r.Volume, r.OpenInterest, r.DollarVolume, r.DollarOpenInterest, r.SID)
	}

	results := w.db.SendBatch(w.ctx, batch)
	defer results.Close()

	for range rows {
		ct, err := results.Exec()
		if err != nil {
			return 0, err
		}
		if ct.RowsAffected() == 0 {
			conflicts++
		}
	}

	return conflicts, nil
}
