package writer

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/rickgao/kalshi-data/internal/router"
)

func TestOrderbookWriter_TransformDelta(t *testing.T) {
	cfg := DefaultWriterConfig()
	input := router.NewGrowableBuffer[router.OrderbookMsg](10)
	w := NewOrderbookWriter(cfg, input, nil, nil)

	receivedAt := time.Date(2024, 1, 15, 12, 0, 0, 0, time.UTC)
	msg := router.OrderbookMsg{
		Type:         "delta",
		Ticker:       "AAPL-JAN-100",
		SID:          1001,
		Seq:          42,
		ReceivedAt:   receivedAt,
		PriceDollars: "0.52",
		Delta:        100,
		Side:         "yes",
		ExchangeTs:   1705320000000000,
	}

	row := w.transformDelta(msg)

	if row.Ticker != "AAPL-JAN-100" {
		t.Errorf("Ticker = %s, want AAPL-JAN-100", row.Ticker)
	}
	if row.ExchangeTs != 1705320000000000 {
		t.Errorf("ExchangeTs = %d, want 1705320000000000", row.ExchangeTs)
	}
	if row.ReceivedAt != receivedAt.UnixMicro() {
		t.Errorf("ReceivedAt = %d, want %d", row.ReceivedAt, receivedAt.UnixMicro())
	}
	if row.Seq != 42 {
		t.Errorf("Seq = %d, want 42", row.Seq)
	}
	if row.Side != true {
		t.Errorf("Side = %v, want true (yes)", row.Side)
	}
	if row.Price != 52000 {
		t.Errorf("Price = %d, want 52000", row.Price)
	}
	if row.SizeDelta != 100 {
		t.Errorf("SizeDelta = %d, want 100", row.SizeDelta)
	}
	if row.SID != 1001 {
		t.Errorf("SID = %d, want 1001", row.SID)
	}
}

func TestOrderbookWriter_TransformDelta_NoSide(t *testing.T) {
	cfg := DefaultWriterConfig()
	input := router.NewGrowableBuffer[router.OrderbookMsg](10)
	w := NewOrderbookWriter(cfg, input, nil, nil)

	msg := router.OrderbookMsg{
		Type:       "delta",
		Side:       "no",
		ReceivedAt: time.Now(),
	}

	row := w.transformDelta(msg)

	if row.Side != false {
		t.Errorf("Side = %v, want false for 'no'", row.Side)
	}
}

func TestOrderbookWriter_TransformSnapshot(t *testing.T) {
	cfg := DefaultWriterConfig()
	input := router.NewGrowableBuffer[router.OrderbookMsg](10)
	w := NewOrderbookWriter(cfg, input, nil, nil)

	receivedAt := time.Date(2024, 1, 15, 12, 0, 0, 0, time.UTC)
	msg := router.OrderbookMsg{
		Type:       "snapshot",
		Ticker:     "AAPL-JAN-100",
		SID:        1001,
		Seq:        1,
		ReceivedAt: receivedAt,
		Yes: []router.PriceLevel{
			{Dollars: "0.52", Quantity: 100},
			{Dollars: "0.51", Quantity: 200},
		},
		No: []router.PriceLevel{
			{Dollars: "0.48", Quantity: 150},
			{Dollars: "0.47", Quantity: 250},
		},
	}

	row := w.transformSnapshot(msg)

	if row.Ticker != "AAPL-JAN-100" {
		t.Errorf("Ticker = %s, want AAPL-JAN-100", row.Ticker)
	}
	if row.SnapshotTs != receivedAt.UnixMicro() {
		t.Errorf("SnapshotTs = %d, want %d", row.SnapshotTs, receivedAt.UnixMicro())
	}
	if row.ExchangeTs != 0 {
		t.Errorf("ExchangeTs = %d, want 0 for WS snapshot", row.ExchangeTs)
	}
	if row.Source != "ws" {
		t.Errorf("Source = %s, want ws", row.Source)
	}
	if row.SID != 1001 {
		t.Errorf("SID = %d, want 1001", row.SID)
	}

	// Verify best prices
	if row.BestYesBid != 52000 {
		t.Errorf("BestYesBid = %d, want 52000", row.BestYesBid)
	}
	// Best YES ask = 100000 - best NO bid = 100000 - 48000 = 52000
	if row.BestYesAsk != 52000 {
		t.Errorf("BestYesAsk = %d, want 52000", row.BestYesAsk)
	}
	if row.Spread != 0 { // Best bid == best ask in this case
		t.Errorf("Spread = %d, want 0", row.Spread)
	}

	// Verify JSONB data
	var yesBids []priceLevelJSON
	if err := json.Unmarshal(row.YesBids, &yesBids); err != nil {
		t.Fatalf("failed to unmarshal YesBids: %v", err)
	}
	if len(yesBids) != 2 {
		t.Errorf("YesBids has %d levels, want 2", len(yesBids))
	}
	if yesBids[0].Price != 52000 || yesBids[0].Size != 100 {
		t.Errorf("YesBids[0] = {%d, %d}, want {52000, 100}", yesBids[0].Price, yesBids[0].Size)
	}

	// YES asks derived from NO bids
	var yesAsks []priceLevelJSON
	if err := json.Unmarshal(row.YesAsks, &yesAsks); err != nil {
		t.Fatalf("failed to unmarshal YesAsks: %v", err)
	}
	if len(yesAsks) != 2 {
		t.Errorf("YesAsks has %d levels, want 2", len(yesAsks))
	}
	// NO bid at 48000 → YES ask at 52000
	if yesAsks[0].Price != 52000 || yesAsks[0].Size != 150 {
		t.Errorf("YesAsks[0] = {%d, %d}, want {52000, 150}", yesAsks[0].Price, yesAsks[0].Size)
	}
}

func TestOrderbookWriter_TransformSnapshot_Empty(t *testing.T) {
	cfg := DefaultWriterConfig()
	input := router.NewGrowableBuffer[router.OrderbookMsg](10)
	w := NewOrderbookWriter(cfg, input, nil, nil)

	msg := router.OrderbookMsg{
		Type:       "snapshot",
		Ticker:     "EMPTY-BOOK",
		ReceivedAt: time.Now(),
		Yes:        nil,
		No:         nil,
	}

	row := w.transformSnapshot(msg)

	if row.BestYesBid != 0 {
		t.Errorf("BestYesBid = %d, want 0 for empty book", row.BestYesBid)
	}
	if row.BestYesAsk != 0 {
		t.Errorf("BestYesAsk = %d, want 0 for empty book", row.BestYesAsk)
	}
	if row.Spread != 0 {
		t.Errorf("Spread = %d, want 0 for empty book", row.Spread)
	}
}

func TestOrderbookWriter_Lifecycle(t *testing.T) {
	cfg := WriterConfig{
		BatchSize:     10,
		FlushInterval: 100 * time.Millisecond,
	}
	input := router.NewGrowableBuffer[router.OrderbookMsg](10)

	w := NewOrderbookWriter(cfg, input, nil, nil)

	ctx := context.Background()
	if err := w.Start(ctx); err != nil {
		t.Fatalf("Start() error = %v", err)
	}

	time.Sleep(20 * time.Millisecond)

	stopCtx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()

	if err := w.Stop(stopCtx); err != nil {
		t.Errorf("Stop() error = %v", err)
	}
}

func TestOrderbookWriter_HandleMessage_Delta(t *testing.T) {
	cfg := WriterConfig{
		BatchSize:     100,
		FlushInterval: time.Hour,
	}
	input := router.NewGrowableBuffer[router.OrderbookMsg](10)
	w := NewOrderbookWriter(cfg, input, nil, nil)

	msg := router.OrderbookMsg{
		Type:         "delta",
		Ticker:       "TEST",
		PriceDollars: "0.50",
		Delta:        10,
		Side:         "yes",
		ReceivedAt:   time.Now(),
	}

	w.handleMessage(msg)

	w.batchMu.Lock()
	deltaLen := len(w.deltaBatch)
	snapshotLen := len(w.snapshotBatch)
	w.batchMu.Unlock()

	if deltaLen != 1 {
		t.Errorf("deltaBatch length = %d, want 1", deltaLen)
	}
	if snapshotLen != 0 {
		t.Errorf("snapshotBatch length = %d, want 0", snapshotLen)
	}
}

func TestOrderbookWriter_HandleMessage_Snapshot(t *testing.T) {
	cfg := WriterConfig{
		BatchSize:     100,
		FlushInterval: time.Hour,
	}
	input := router.NewGrowableBuffer[router.OrderbookMsg](10)
	w := NewOrderbookWriter(cfg, input, nil, nil)

	msg := router.OrderbookMsg{
		Type:       "snapshot",
		Ticker:     "TEST",
		ReceivedAt: time.Now(),
		Yes:        []router.PriceLevel{{Dollars: "0.50", Quantity: 100}},
	}

	w.handleMessage(msg)

	w.batchMu.Lock()
	deltaLen := len(w.deltaBatch)
	snapshotLen := len(w.snapshotBatch)
	w.batchMu.Unlock()

	if deltaLen != 0 {
		t.Errorf("deltaBatch length = %d, want 0", deltaLen)
	}
	if snapshotLen != 1 {
		t.Errorf("snapshotBatch length = %d, want 1", snapshotLen)
	}
}

func TestOrderbookWriter_HandleMessage_SeqGap(t *testing.T) {
	cfg := DefaultWriterConfig()
	input := router.NewGrowableBuffer[router.OrderbookMsg](10)
	w := NewOrderbookWriter(cfg, input, nil, nil)

	msg := router.OrderbookMsg{
		Type:       "delta",
		Ticker:     "TEST",
		SeqGap:     true,
		GapSize:    5,
		ReceivedAt: time.Now(),
	}

	w.handleMessage(msg)

	stats := w.Stats()
	if stats.SeqGaps != 1 {
		t.Errorf("SeqGaps = %d, want 1", stats.SeqGaps)
	}
}

func TestOrderbookWriter_Stats(t *testing.T) {
	cfg := DefaultWriterConfig()
	input := router.NewGrowableBuffer[router.OrderbookMsg](10)
	w := NewOrderbookWriter(cfg, input, nil, nil)

	stats := w.Stats()

	if stats.DeltaInserts != 0 {
		t.Errorf("initial DeltaInserts = %d, want 0", stats.DeltaInserts)
	}
	if stats.SnapshotInserts != 0 {
		t.Errorf("initial SnapshotInserts = %d, want 0", stats.SnapshotInserts)
	}
}
