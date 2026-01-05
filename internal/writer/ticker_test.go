package writer

import (
	"context"
	"testing"
	"time"

	"github.com/rickgao/kalshi-data/internal/router"
)

func TestTickerWriter_Transform(t *testing.T) {
	cfg := DefaultWriterConfig()
	input := router.NewGrowableBuffer[router.TickerMsg](10)
	w := NewTickerWriter(cfg, input, nil, nil)

	receivedAt := time.Date(2024, 1, 15, 12, 0, 0, 0, time.UTC)
	msg := router.TickerMsg{
		Ticker:             "AAPL-JAN-100",
		PriceDollars:       "0.52",
		YesBidDollars:      "0.51",
		YesAskDollars:      "0.53",
		NoBidDollars:       "0.47",
		Volume:             1000,
		OpenInterest:       500,
		DollarVolume:       52000,
		DollarOpenInterest: 26000,
		SID:                1001,
		ExchangeTs:         1705320000000000,
		ReceivedAt:         receivedAt,
	}

	row := w.transform(msg)

	if row.Ticker != "AAPL-JAN-100" {
		t.Errorf("Ticker = %s, want AAPL-JAN-100", row.Ticker)
	}
	if row.ExchangeTs != 1705320000000000 {
		t.Errorf("ExchangeTs = %d, want 1705320000000000", row.ExchangeTs)
	}
	if row.ReceivedAt != receivedAt.UnixMicro() {
		t.Errorf("ReceivedAt = %d, want %d", row.ReceivedAt, receivedAt.UnixMicro())
	}
	if row.LastPrice != 52000 {
		t.Errorf("LastPrice = %d, want 52000", row.LastPrice)
	}
	if row.YesBid != 51000 {
		t.Errorf("YesBid = %d, want 51000", row.YesBid)
	}
	if row.YesAsk != 53000 {
		t.Errorf("YesAsk = %d, want 53000", row.YesAsk)
	}
	if row.Volume != 1000 {
		t.Errorf("Volume = %d, want 1000", row.Volume)
	}
	if row.OpenInterest != 500 {
		t.Errorf("OpenInterest = %d, want 500", row.OpenInterest)
	}
	if row.DollarVolume != 52000 {
		t.Errorf("DollarVolume = %d, want 52000", row.DollarVolume)
	}
	if row.DollarOpenInterest != 26000 {
		t.Errorf("DollarOpenInterest = %d, want 26000", row.DollarOpenInterest)
	}
	if row.SID != 1001 {
		t.Errorf("SID = %d, want 1001", row.SID)
	}
}

func TestTickerWriter_Transform_EmptyPrices(t *testing.T) {
	cfg := DefaultWriterConfig()
	input := router.NewGrowableBuffer[router.TickerMsg](10)
	w := NewTickerWriter(cfg, input, nil, nil)

	msg := router.TickerMsg{
		ReceivedAt: time.Now(),
		// All price fields empty
	}

	row := w.transform(msg)

	if row.LastPrice != 0 {
		t.Errorf("LastPrice = %d, want 0 for empty", row.LastPrice)
	}
	if row.YesBid != 0 {
		t.Errorf("YesBid = %d, want 0 for empty", row.YesBid)
	}
	if row.YesAsk != 0 {
		t.Errorf("YesAsk = %d, want 0 for empty", row.YesAsk)
	}
}

func TestTickerWriter_Lifecycle(t *testing.T) {
	cfg := WriterConfig{
		BatchSize:     10,
		FlushInterval: 100 * time.Millisecond,
	}
	input := router.NewGrowableBuffer[router.TickerMsg](10)

	w := NewTickerWriter(cfg, input, nil, nil)

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

func TestTickerWriter_HandleMessage_AddsToBatch(t *testing.T) {
	cfg := WriterConfig{
		BatchSize:     100,
		FlushInterval: time.Hour,
	}
	input := router.NewGrowableBuffer[router.TickerMsg](10)
	w := NewTickerWriter(cfg, input, nil, nil)

	msg := router.TickerMsg{
		Ticker:        "TEST",
		PriceDollars:  "0.50",
		YesBidDollars: "0.49",
		YesAskDollars: "0.51",
		ReceivedAt:    time.Now(),
	}

	w.handleMessage(msg)

	w.batchMu.Lock()
	batchLen := len(w.batch)
	w.batchMu.Unlock()

	if batchLen != 1 {
		t.Errorf("batch length = %d, want 1", batchLen)
	}
}

func TestTickerWriter_Stats(t *testing.T) {
	cfg := DefaultWriterConfig()
	input := router.NewGrowableBuffer[router.TickerMsg](10)
	w := NewTickerWriter(cfg, input, nil, nil)

	stats := w.Stats()

	if stats.Inserts != 0 {
		t.Errorf("initial Inserts = %d, want 0", stats.Inserts)
	}
	if stats.Errors != 0 {
		t.Errorf("initial Errors = %d, want 0", stats.Errors)
	}
	if stats.Flushes != 0 {
		t.Errorf("initial Flushes = %d, want 0", stats.Flushes)
	}
}

func TestDefaultWriterConfig(t *testing.T) {
	cfg := DefaultWriterConfig()

	if cfg.BatchSize != 1000 {
		t.Errorf("BatchSize = %d, want 1000", cfg.BatchSize)
	}
	if cfg.FlushInterval != 5*time.Second {
		t.Errorf("FlushInterval = %v, want 5s", cfg.FlushInterval)
	}
}
