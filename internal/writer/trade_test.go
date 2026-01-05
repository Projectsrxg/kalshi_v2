package writer

import (
	"context"
	"testing"
	"time"

	"github.com/rickgao/kalshi-data/internal/router"
)

func TestTradeWriter_Transform(t *testing.T) {
	cfg := DefaultWriterConfig()
	input := router.NewGrowableBuffer[router.TradeMsg](10)
	w := NewTradeWriter(cfg, input, nil, nil)

	receivedAt := time.Date(2024, 1, 15, 12, 0, 0, 0, time.UTC)
	msg := router.TradeMsg{
		Ticker:          "AAPL-JAN-100",
		TradeID:         "trade-123",
		Size:            50,
		YesPriceDollars: "0.52",
		NoPriceDollars:  "0.48",
		TakerSide:       "yes",
		SID:             1001,
		Seq:             42,
		ExchangeTs:      1705320000000000, // microseconds
		ReceivedAt:      receivedAt,
	}

	row := w.transform(msg)

	if row.TradeID != "trade-123" {
		t.Errorf("TradeID = %s, want trade-123", row.TradeID)
	}
	if row.ExchangeTs != 1705320000000000 {
		t.Errorf("ExchangeTs = %d, want 1705320000000000", row.ExchangeTs)
	}
	if row.ReceivedAt != receivedAt.UnixMicro() {
		t.Errorf("ReceivedAt = %d, want %d", row.ReceivedAt, receivedAt.UnixMicro())
	}
	if row.Ticker != "AAPL-JAN-100" {
		t.Errorf("Ticker = %s, want AAPL-JAN-100", row.Ticker)
	}
	if row.Price != 52000 {
		t.Errorf("Price = %d, want 52000", row.Price)
	}
	if row.Size != 50 {
		t.Errorf("Size = %d, want 50", row.Size)
	}
	if row.TakerSide != true {
		t.Errorf("TakerSide = %v, want true", row.TakerSide)
	}
	if row.SID != 1001 {
		t.Errorf("SID = %d, want 1001", row.SID)
	}
}

func TestTradeWriter_Transform_NoSide(t *testing.T) {
	cfg := DefaultWriterConfig()
	input := router.NewGrowableBuffer[router.TradeMsg](10)
	w := NewTradeWriter(cfg, input, nil, nil)

	msg := router.TradeMsg{
		TakerSide: "no",
	}

	row := w.transform(msg)

	if row.TakerSide != false {
		t.Errorf("TakerSide = %v, want false for 'no' side", row.TakerSide)
	}
}

func TestTradeWriter_Lifecycle(t *testing.T) {
	cfg := WriterConfig{
		BatchSize:     10,
		FlushInterval: 100 * time.Millisecond,
	}
	input := router.NewGrowableBuffer[router.TradeMsg](10)

	// Note: We can't test actual DB writes without a database
	// This tests the goroutine lifecycle
	w := NewTradeWriter(cfg, input, nil, nil)

	ctx := context.Background()
	if err := w.Start(ctx); err != nil {
		t.Fatalf("Start() error = %v", err)
	}

	// Give goroutines time to start
	time.Sleep(20 * time.Millisecond)

	// Stop should complete without hanging
	stopCtx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()

	if err := w.Stop(stopCtx); err != nil {
		t.Errorf("Stop() error = %v", err)
	}
}

func TestTradeWriter_HandleMessage_AddsToBatch(t *testing.T) {
	cfg := WriterConfig{
		BatchSize:     100, // Large batch so no auto-flush
		FlushInterval: time.Hour,
	}
	input := router.NewGrowableBuffer[router.TradeMsg](10)
	w := NewTradeWriter(cfg, input, nil, nil)

	// Manually call handleMessage to test batching
	msg := router.TradeMsg{
		TradeID:         "trade-1",
		YesPriceDollars: "0.50",
		TakerSide:       "yes",
		ReceivedAt:      time.Now(),
	}

	w.handleMessage(msg)

	w.batchMu.Lock()
	batchLen := len(w.batch)
	w.batchMu.Unlock()

	if batchLen != 1 {
		t.Errorf("batch length = %d, want 1", batchLen)
	}
}

func TestTradeWriter_Stats(t *testing.T) {
	cfg := DefaultWriterConfig()
	input := router.NewGrowableBuffer[router.TradeMsg](10)
	w := NewTradeWriter(cfg, input, nil, nil)

	stats := w.Stats()

	if stats.Inserts != 0 {
		t.Errorf("initial Inserts = %d, want 0", stats.Inserts)
	}
	if stats.Errors != 0 {
		t.Errorf("initial Errors = %d, want 0", stats.Errors)
	}
}
