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

// OrderbookWriter consumes OrderbookMsg from the router buffer and writes
// to orderbook_deltas and orderbook_snapshots tables.
type OrderbookWriter struct {
	cfg    WriterConfig
	logger *slog.Logger

	// Input from Message Router
	input *router.GrowableBuffer[router.OrderbookMsg]

	// Database
	db *pgxpool.Pool

	// Batching (separate batches for deltas and snapshots)
	deltaBatch    []orderbookDeltaRow
	snapshotBatch []orderbookSnapshotRow
	batchMu       sync.Mutex
	flushTicker   *time.Ticker

	// Lifecycle
	ctx    context.Context
	cancel context.CancelFunc
	wg     sync.WaitGroup

	// Metrics
	metrics OrderbookWriterMetrics
}

// OrderbookWriterMetrics extends WriterMetrics with delta/snapshot breakdown.
type OrderbookWriterMetrics struct {
	DeltaInserts    int64
	DeltaConflicts  int64
	DeltaErrors     int64
	SnapshotInserts int64
	SnapshotErrors  int64
	SeqGaps         int64
	Flushes         int64
}

// NewOrderbookWriter creates a new OrderbookWriter.
func NewOrderbookWriter(
	cfg WriterConfig,
	input *router.GrowableBuffer[router.OrderbookMsg],
	db *pgxpool.Pool,
	logger *slog.Logger,
) *OrderbookWriter {
	if logger == nil {
		logger = slog.Default()
	}
	return &OrderbookWriter{
		cfg:           cfg,
		input:         input,
		db:            db,
		logger:        logger,
		deltaBatch:    make([]orderbookDeltaRow, 0, cfg.BatchSize),
		snapshotBatch: make([]orderbookSnapshotRow, 0, 100), // Snapshots are less frequent
	}
}

// Start begins consuming messages and writing to the database.
func (w *OrderbookWriter) Start(ctx context.Context) error {
	w.ctx, w.cancel = context.WithCancel(ctx)
	w.flushTicker = time.NewTicker(w.cfg.FlushInterval)

	// Consumer goroutine
	w.wg.Add(1)
	go w.consumeLoop()

	// Flush ticker goroutine
	w.wg.Add(1)
	go w.flushLoop()

	w.logger.Info("orderbook writer started",
		"batch_size", w.cfg.BatchSize,
		"flush_interval", w.cfg.FlushInterval,
	)
	return nil
}

// Stop gracefully shuts down the writer.
func (w *OrderbookWriter) Stop(ctx context.Context) error {
	w.logger.Info("stopping orderbook writer")

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
		w.logger.Info("orderbook writer stopped")
	case <-ctx.Done():
		w.logger.Warn("orderbook writer stop timed out")
	}

	// Final flush
	w.flush()

	return nil
}

// Stats returns current metrics.
func (w *OrderbookWriter) Stats() OrderbookWriterMetrics {
	w.batchMu.Lock()
	defer w.batchMu.Unlock()
	return w.metrics
}

// consumeLoop reads from the input buffer and accumulates batches.
func (w *OrderbookWriter) consumeLoop() {
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

// flushLoop periodically flushes the batches.
func (w *OrderbookWriter) flushLoop() {
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

// handleMessage processes a message based on its type (snapshot or delta).
func (w *OrderbookWriter) handleMessage(msg router.OrderbookMsg) {
	// Track sequence gaps
	if msg.SeqGap {
		w.logger.Warn("sequence gap detected",
			"ticker", msg.Ticker,
			"gap_size", msg.GapSize,
		)
		w.batchMu.Lock()
		w.metrics.SeqGaps++
		w.batchMu.Unlock()
	}

	switch msg.Type {
	case "snapshot":
		row := w.transformSnapshot(msg)
		w.batchMu.Lock()
		w.snapshotBatch = append(w.snapshotBatch, row)
		w.batchMu.Unlock()
	case "delta":
		row := w.transformDelta(msg)
		w.batchMu.Lock()
		w.deltaBatch = append(w.deltaBatch, row)
		shouldFlush := len(w.deltaBatch) >= w.cfg.BatchSize
		w.batchMu.Unlock()
		if shouldFlush {
			w.flush()
		}
	default:
		w.logger.Warn("unknown orderbook message type", "type", msg.Type)
	}
}

// transformDelta converts an OrderbookMsg (delta) to orderbookDeltaRow.
func (w *OrderbookWriter) transformDelta(msg router.OrderbookMsg) orderbookDeltaRow {
	return orderbookDeltaRow{
		ExchangeTs: msg.ExchangeTs,
		ReceivedAt: msg.ReceivedAt.UnixMicro(),
		Seq:        msg.Seq,
		Ticker:     msg.Ticker,
		Side:       sideToBoolean(msg.Side),
		Price:      dollarsToInternal(msg.PriceDollars),
		SizeDelta:  msg.Delta,
		SID:        msg.SID,
	}
}

// transformSnapshot converts an OrderbookMsg (snapshot) to orderbookSnapshotRow.
func (w *OrderbookWriter) transformSnapshot(msg router.OrderbookMsg) orderbookSnapshotRow {
	yesBids := priceLevelsToJSONB(msg.Yes)
	noBids := priceLevelsToJSONB(msg.No)

	// Derive asks from opposite bids
	// YES bid at price X means NO ask at (100000 - X)
	yesAsks := deriveAsksFromBids(msg.No) // NO bids → YES asks
	noAsks := deriveAsksFromBids(msg.Yes) // YES bids → NO asks

	bestYesBid := extractBestPrice(msg.Yes)
	bestYesAsk := extractBestAskFromBids(msg.No)

	spread := 0
	if bestYesBid > 0 && bestYesAsk > 0 {
		spread = bestYesAsk - bestYesBid
	}

	return orderbookSnapshotRow{
		SnapshotTs: msg.ReceivedAt.UnixMicro(),
		ExchangeTs: 0, // WS snapshots don't have exchange timestamp
		Ticker:     msg.Ticker,
		Source:     "ws",
		YesBids:    yesBids,
		YesAsks:    yesAsks,
		NoBids:     noBids,
		NoAsks:     noAsks,
		BestYesBid: bestYesBid,
		BestYesAsk: bestYesAsk,
		Spread:     spread,
		SID:        msg.SID,
	}
}

// flush writes both batches to the database.
func (w *OrderbookWriter) flush() {
	w.batchMu.Lock()
	deltaBatch := w.deltaBatch
	snapshotBatch := w.snapshotBatch
	w.deltaBatch = make([]orderbookDeltaRow, 0, w.cfg.BatchSize)
	w.snapshotBatch = make([]orderbookSnapshotRow, 0, 100)
	w.batchMu.Unlock()

	if len(deltaBatch) == 0 && len(snapshotBatch) == 0 {
		return
	}

	start := time.Now()

	// Flush deltas
	if len(deltaBatch) > 0 {
		conflicts, err := w.batchInsertDeltas(deltaBatch)
		if err != nil {
			w.logger.Error("delta batch insert failed", "error", err, "count", len(deltaBatch))
			w.batchMu.Lock()
			w.metrics.DeltaErrors++
			w.batchMu.Unlock()
		} else {
			w.batchMu.Lock()
			w.metrics.DeltaInserts += int64(len(deltaBatch) - conflicts)
			w.metrics.DeltaConflicts += int64(conflicts)
			w.batchMu.Unlock()
		}
	}

	// Flush snapshots
	if len(snapshotBatch) > 0 {
		err := w.batchInsertSnapshots(snapshotBatch)
		if err != nil {
			w.logger.Error("snapshot batch insert failed", "error", err, "count", len(snapshotBatch))
			w.batchMu.Lock()
			w.metrics.SnapshotErrors++
			w.batchMu.Unlock()
		} else {
			w.batchMu.Lock()
			w.metrics.SnapshotInserts += int64(len(snapshotBatch))
			w.batchMu.Unlock()
		}
	}

	w.batchMu.Lock()
	w.metrics.Flushes++
	w.batchMu.Unlock()

	w.logger.Debug("flushed orderbook",
		"deltas", len(deltaBatch),
		"snapshots", len(snapshotBatch),
		"duration", time.Since(start),
	)
}

// batchInsertDeltas inserts delta rows with ON CONFLICT DO NOTHING.
func (w *OrderbookWriter) batchInsertDeltas(rows []orderbookDeltaRow) (conflicts int, err error) {
	batch := &pgx.Batch{}
	for _, r := range rows {
		batch.Queue(`
			INSERT INTO orderbook_deltas (exchange_ts, received_at, ticker, side, price, size_delta, seq, sid)
			VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
			ON CONFLICT (ticker, exchange_ts, price, side) DO NOTHING
		`, r.ExchangeTs, r.ReceivedAt, r.Ticker, r.Side, r.Price, r.SizeDelta, r.Seq, r.SID)
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

// batchInsertSnapshots inserts snapshot rows with ON CONFLICT DO NOTHING.
func (w *OrderbookWriter) batchInsertSnapshots(rows []orderbookSnapshotRow) error {
	batch := &pgx.Batch{}
	for _, r := range rows {
		batch.Queue(`
			INSERT INTO orderbook_snapshots (snapshot_ts, exchange_ts, ticker, source, yes_bids, yes_asks, no_bids, no_asks, best_yes_bid, best_yes_ask, spread, sid)
			VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12)
			ON CONFLICT (ticker, snapshot_ts, source) DO NOTHING
		`, r.SnapshotTs, r.ExchangeTs, r.Ticker, r.Source, r.YesBids, r.YesAsks, r.NoBids, r.NoAsks, r.BestYesBid, r.BestYesAsk, r.Spread, r.SID)
	}

	results := w.db.SendBatch(w.ctx, batch)
	defer results.Close()

	for range rows {
		if _, err := results.Exec(); err != nil {
			return err
		}
	}

	return nil
}
