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

// TradeWriter consumes TradeMsg from the router buffer and writes to the trades table.
type TradeWriter struct {
	cfg    WriterConfig
	logger *slog.Logger

	// Input from Message Router
	input *router.GrowableBuffer[router.TradeMsg]

	// Database
	db *pgxpool.Pool

	// Batching
	batch       []tradeRow
	batchMu     sync.Mutex
	flushTicker *time.Ticker

	// Lifecycle
	ctx    context.Context
	cancel context.CancelFunc
	wg     sync.WaitGroup

	// Metrics
	metrics WriterMetrics
}

// NewTradeWriter creates a new TradeWriter.
func NewTradeWriter(
	cfg WriterConfig,
	input *router.GrowableBuffer[router.TradeMsg],
	db *pgxpool.Pool,
	logger *slog.Logger,
) *TradeWriter {
	if logger == nil {
		logger = slog.Default()
	}
	return &TradeWriter{
		cfg:    cfg,
		input:  input,
		db:     db,
		logger: logger,
		batch:  make([]tradeRow, 0, cfg.BatchSize),
	}
}

// Start begins consuming messages and writing to the database.
func (w *TradeWriter) Start(ctx context.Context) error {
	w.ctx, w.cancel = context.WithCancel(ctx)
	w.flushTicker = time.NewTicker(w.cfg.FlushInterval)

	// Consumer goroutine
	w.wg.Add(1)
	go w.consumeLoop()

	// Flush ticker goroutine
	w.wg.Add(1)
	go w.flushLoop()

	w.logger.Info("trade writer started",
		"batch_size", w.cfg.BatchSize,
		"flush_interval", w.cfg.FlushInterval,
	)
	return nil
}

// Stop gracefully shuts down the writer.
func (w *TradeWriter) Stop(ctx context.Context) error {
	w.logger.Info("stopping trade writer")

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
		w.logger.Info("trade writer stopped")
	case <-ctx.Done():
		w.logger.Warn("trade writer stop timed out")
	}

	// Final flush
	w.flush()

	return nil
}

// Stats returns current metrics.
func (w *TradeWriter) Stats() WriterMetrics {
	w.batchMu.Lock()
	defer w.batchMu.Unlock()
	return w.metrics
}

// consumeLoop reads from the input buffer and accumulates batches.
func (w *TradeWriter) consumeLoop() {
	defer w.wg.Done()

	for {
		select {
		case <-w.ctx.Done():
			return
		default:
			// Use TryReceive with context check for responsiveness
			msg, ok := w.input.TryReceive()
			if !ok {
				// Buffer empty, wait a bit before trying again
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
func (w *TradeWriter) flushLoop() {
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
func (w *TradeWriter) handleMessage(msg router.TradeMsg) {
	row := w.transform(msg)

	w.batchMu.Lock()
	w.batch = append(w.batch, row)
	shouldFlush := len(w.batch) >= w.cfg.BatchSize
	w.batchMu.Unlock()

	if shouldFlush {
		w.flush()
	}
}

// transform converts a TradeMsg to a tradeRow.
func (w *TradeWriter) transform(msg router.TradeMsg) tradeRow {
	return tradeRow{
		TradeID:    msg.TradeID,
		ExchangeTs: msg.ExchangeTs,
		ReceivedAt: msg.ReceivedAt.UnixMicro(),
		Ticker:     msg.Ticker,
		Price:      dollarsToInternal(msg.YesPriceDollars),
		Size:       msg.Size,
		TakerSide:  sideToBoolean(msg.TakerSide),
		SID:        msg.SID,
	}
}

// flush writes the current batch to the database.
func (w *TradeWriter) flush() {
	w.batchMu.Lock()
	if len(w.batch) == 0 {
		w.batchMu.Unlock()
		return
	}

	// Take ownership of current batch
	batch := w.batch
	w.batch = make([]tradeRow, 0, w.cfg.BatchSize)
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

	w.logger.Debug("flushed trades",
		"count", len(batch),
		"conflicts", conflicts,
		"duration", time.Since(start),
	)
}

// batchInsert inserts rows using pgx.Batch with ON CONFLICT DO NOTHING.
func (w *TradeWriter) batchInsert(rows []tradeRow) (conflicts int, err error) {
	batch := &pgx.Batch{}
	for _, r := range rows {
		batch.Queue(`
			INSERT INTO trades (trade_id, exchange_ts, received_at, ticker, price, size, taker_side, sid)
			VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
			ON CONFLICT (trade_id) DO NOTHING
		`, r.TradeID, r.ExchangeTs, r.ReceivedAt, r.Ticker, r.Price, r.Size, r.TakerSide, r.SID)
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
