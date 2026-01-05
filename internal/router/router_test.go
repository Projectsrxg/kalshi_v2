package router

import (
	"context"
	"encoding/json"
	"log/slog"
	"testing"
	"time"

	"github.com/rickgao/kalshi-data/internal/connection"
)

func TestDefaultRouterConfig(t *testing.T) {
	cfg := DefaultRouterConfig()

	if cfg.OrderbookBufferSize != 5000 {
		t.Errorf("OrderbookBufferSize = %d, want 5000", cfg.OrderbookBufferSize)
	}
	if cfg.TradeBufferSize != 1000 {
		t.Errorf("TradeBufferSize = %d, want 1000", cfg.TradeBufferSize)
	}
	if cfg.TickerBufferSize != 1000 {
		t.Errorf("TickerBufferSize = %d, want 1000", cfg.TickerBufferSize)
	}
}

func TestRouter_StartStop(t *testing.T) {
	input := make(chan connection.RawMessage, 10)
	cfg := DefaultRouterConfig()
	r := NewRouter(cfg, input, slog.Default())

	ctx := context.Background()
	if err := r.Start(ctx); err != nil {
		t.Fatalf("Start failed: %v", err)
	}

	// Give it a moment to start
	time.Sleep(10 * time.Millisecond)

	stopCtx, cancel := context.WithTimeout(ctx, time.Second)
	defer cancel()

	if err := r.Stop(stopCtx); err != nil {
		t.Errorf("Stop failed: %v", err)
	}
}

func TestRouter_ParseOrderbookDelta(t *testing.T) {
	input := make(chan connection.RawMessage, 10)
	cfg := DefaultRouterConfig()
	r := NewRouter(cfg, input, slog.Default())

	ctx := context.Background()
	if err := r.Start(ctx); err != nil {
		t.Fatalf("Start failed: %v", err)
	}
	defer r.Stop(ctx)

	// Send an orderbook delta message
	deltaMsg := map[string]interface{}{
		"type": "orderbook_delta",
		"sid":  1,
		"seq":  100,
		"msg": map[string]interface{}{
			"market_ticker": "TEST-MARKET",
			"price_dollars": "0.52",
			"delta":         10,
			"side":          "yes",
			"ts":            1705328200,
		},
	}
	data, _ := json.Marshal(deltaMsg)

	input <- connection.RawMessage{
		Data:       data,
		ConnID:     1,
		ReceivedAt: time.Now(),
		SeqGap:     false,
		GapSize:    0,
	}

	// Wait for routing
	time.Sleep(50 * time.Millisecond)

	buffers := r.Buffers()
	msg, ok := buffers.Orderbook.TryReceive()
	if !ok {
		t.Fatal("expected orderbook message")
	}

	if msg.Type != "delta" {
		t.Errorf("Type = %s, want delta", msg.Type)
	}
	if msg.Ticker != "TEST-MARKET" {
		t.Errorf("Ticker = %s, want TEST-MARKET", msg.Ticker)
	}
	if msg.PriceDollars != "0.52" {
		t.Errorf("PriceDollars = %s, want 0.52", msg.PriceDollars)
	}
	if msg.Delta != 10 {
		t.Errorf("Delta = %d, want 10", msg.Delta)
	}
	if msg.Side != "yes" {
		t.Errorf("Side = %s, want yes", msg.Side)
	}
	if msg.ExchangeTs != 1705328200000000 {
		t.Errorf("ExchangeTs = %d, want 1705328200000000", msg.ExchangeTs)
	}
	if msg.SID != 1 {
		t.Errorf("SID = %d, want 1", msg.SID)
	}
	if msg.Seq != 100 {
		t.Errorf("Seq = %d, want 100", msg.Seq)
	}
}

func TestRouter_ParseOrderbookSnapshot(t *testing.T) {
	input := make(chan connection.RawMessage, 10)
	cfg := DefaultRouterConfig()
	r := NewRouter(cfg, input, slog.Default())

	ctx := context.Background()
	if err := r.Start(ctx); err != nil {
		t.Fatalf("Start failed: %v", err)
	}
	defer r.Stop(ctx)

	// Send an orderbook snapshot message
	snapshotMsg := map[string]interface{}{
		"type": "orderbook_snapshot",
		"sid":  2,
		"seq":  1,
		"msg": map[string]interface{}{
			"market_ticker": "SNAPSHOT-MARKET",
			"yes_dollars":   [][]interface{}{{"0.52", 100.0}, {"0.51", 200.0}},
			"no_dollars":    [][]interface{}{{"0.48", 150.0}},
		},
	}
	data, _ := json.Marshal(snapshotMsg)

	input <- connection.RawMessage{
		Data:       data,
		ConnID:     1,
		ReceivedAt: time.Now(),
	}

	time.Sleep(50 * time.Millisecond)

	buffers := r.Buffers()
	msg, ok := buffers.Orderbook.TryReceive()
	if !ok {
		t.Fatal("expected orderbook snapshot message")
	}

	if msg.Type != "snapshot" {
		t.Errorf("Type = %s, want snapshot", msg.Type)
	}
	if msg.Ticker != "SNAPSHOT-MARKET" {
		t.Errorf("Ticker = %s, want SNAPSHOT-MARKET", msg.Ticker)
	}
	if len(msg.Yes) != 2 {
		t.Errorf("Yes levels = %d, want 2", len(msg.Yes))
	}
	if len(msg.No) != 1 {
		t.Errorf("No levels = %d, want 1", len(msg.No))
	}
	if msg.Yes[0].Dollars != "0.52" || msg.Yes[0].Quantity != 100 {
		t.Errorf("Yes[0] = %+v, want {0.52, 100}", msg.Yes[0])
	}
	if msg.Yes[1].Dollars != "0.51" || msg.Yes[1].Quantity != 200 {
		t.Errorf("Yes[1] = %+v, want {0.51, 200}", msg.Yes[1])
	}
	if msg.No[0].Dollars != "0.48" || msg.No[0].Quantity != 150 {
		t.Errorf("No[0] = %+v, want {0.48, 150}", msg.No[0])
	}
}

func TestRouter_ParseTrade(t *testing.T) {
	input := make(chan connection.RawMessage, 10)
	cfg := DefaultRouterConfig()
	r := NewRouter(cfg, input, slog.Default())

	ctx := context.Background()
	if err := r.Start(ctx); err != nil {
		t.Fatalf("Start failed: %v", err)
	}
	defer r.Stop(ctx)

	// Send a trade message
	tradeMsg := map[string]interface{}{
		"type": "trade",
		"sid":  3,
		"seq":  50,
		"msg": map[string]interface{}{
			"market_ticker":     "TRADE-MARKET",
			"trade_id":          "trade-123-abc",
			"count":             5,
			"yes_price_dollars": "0.55",
			"no_price_dollars":  "0.45",
			"taker_side":        "yes",
			"ts":                1705328300,
		},
	}
	data, _ := json.Marshal(tradeMsg)

	input <- connection.RawMessage{
		Data:       data,
		ConnID:     2,
		ReceivedAt: time.Now(),
		SeqGap:     true,
		GapSize:    3,
	}

	time.Sleep(50 * time.Millisecond)

	buffers := r.Buffers()
	msg, ok := buffers.Trade.TryReceive()
	if !ok {
		t.Fatal("expected trade message")
	}

	if msg.Ticker != "TRADE-MARKET" {
		t.Errorf("Ticker = %s, want TRADE-MARKET", msg.Ticker)
	}
	if msg.TradeID != "trade-123-abc" {
		t.Errorf("TradeID = %s, want trade-123-abc", msg.TradeID)
	}
	if msg.Size != 5 {
		t.Errorf("Size = %d, want 5", msg.Size)
	}
	if msg.YesPriceDollars != "0.55" {
		t.Errorf("YesPriceDollars = %s, want 0.55", msg.YesPriceDollars)
	}
	if msg.NoPriceDollars != "0.45" {
		t.Errorf("NoPriceDollars = %s, want 0.45", msg.NoPriceDollars)
	}
	if msg.TakerSide != "yes" {
		t.Errorf("TakerSide = %s, want yes", msg.TakerSide)
	}
	if msg.ExchangeTs != 1705328300000000 {
		t.Errorf("ExchangeTs = %d, want 1705328300000000", msg.ExchangeTs)
	}
	if !msg.SeqGap {
		t.Error("SeqGap should be true")
	}
	if msg.GapSize != 3 {
		t.Errorf("GapSize = %d, want 3", msg.GapSize)
	}
}

func TestRouter_ParseTicker(t *testing.T) {
	input := make(chan connection.RawMessage, 10)
	cfg := DefaultRouterConfig()
	r := NewRouter(cfg, input, slog.Default())

	ctx := context.Background()
	if err := r.Start(ctx); err != nil {
		t.Fatalf("Start failed: %v", err)
	}
	defer r.Stop(ctx)

	// Send a ticker message
	tickerMsg := map[string]interface{}{
		"type": "ticker",
		"sid":  4,
		"msg": map[string]interface{}{
			"market_ticker":        "TICKER-MARKET",
			"price_dollars":        "0.60",
			"yes_bid_dollars":      "0.59",
			"yes_ask_dollars":      "0.61",
			"no_bid_dollars":       "0.39",
			"volume":               1000,
			"open_interest":        500,
			"dollar_volume":        50000,
			"dollar_open_interest": 25000,
			"ts":                   1705328400,
		},
	}
	data, _ := json.Marshal(tickerMsg)

	input <- connection.RawMessage{
		Data:       data,
		ConnID:     3,
		ReceivedAt: time.Now(),
	}

	time.Sleep(50 * time.Millisecond)

	buffers := r.Buffers()
	msg, ok := buffers.Ticker.TryReceive()
	if !ok {
		t.Fatal("expected ticker message")
	}

	if msg.Ticker != "TICKER-MARKET" {
		t.Errorf("Ticker = %s, want TICKER-MARKET", msg.Ticker)
	}
	if msg.PriceDollars != "0.60" {
		t.Errorf("PriceDollars = %s, want 0.60", msg.PriceDollars)
	}
	if msg.YesBidDollars != "0.59" {
		t.Errorf("YesBidDollars = %s, want 0.59", msg.YesBidDollars)
	}
	if msg.YesAskDollars != "0.61" {
		t.Errorf("YesAskDollars = %s, want 0.61", msg.YesAskDollars)
	}
	if msg.NoBidDollars != "0.39" {
		t.Errorf("NoBidDollars = %s, want 0.39", msg.NoBidDollars)
	}
	if msg.Volume != 1000 {
		t.Errorf("Volume = %d, want 1000", msg.Volume)
	}
	if msg.OpenInterest != 500 {
		t.Errorf("OpenInterest = %d, want 500", msg.OpenInterest)
	}
	if msg.ExchangeTs != 1705328400000000 {
		t.Errorf("ExchangeTs = %d, want 1705328400000000", msg.ExchangeTs)
	}
}

func TestRouter_BufferGrowth(t *testing.T) {
	input := make(chan connection.RawMessage, 100)
	cfg := RouterConfig{
		OrderbookBufferSize: 10, // Small buffer to trigger growth
		TradeBufferSize:     1000,
		TickerBufferSize:    1000,
	}
	r := NewRouter(cfg, input, slog.Default())

	ctx := context.Background()
	if err := r.Start(ctx); err != nil {
		t.Fatalf("Start failed: %v", err)
	}
	defer r.Stop(ctx)

	// Send more messages than initial buffer size
	for i := 0; i < 50; i++ {
		deltaMsg := map[string]interface{}{
			"type": "orderbook_delta",
			"sid":  1,
			"seq":  int64(i),
			"msg": map[string]interface{}{
				"market_ticker": "GROWTH-TEST",
				"price_dollars": "0.50",
				"delta":         1,
				"side":          "yes",
				"ts":            1705328200,
			},
		}
		data, _ := json.Marshal(deltaMsg)
		input <- connection.RawMessage{
			Data:       data,
			ReceivedAt: time.Now(),
		}
	}

	// Wait for processing
	time.Sleep(100 * time.Millisecond)

	stats := r.Stats()
	if stats.MessagesReceived != 50 {
		t.Errorf("MessagesReceived = %d, want 50", stats.MessagesReceived)
	}
	// All messages should be routed (no drops with growable buffer)
	if stats.MessagesRouted != 50 {
		t.Errorf("MessagesRouted = %d, want 50", stats.MessagesRouted)
	}
	// Buffer should have grown
	if stats.OrderbookBuffer.Capacity <= 10 {
		t.Errorf("Buffer capacity = %d, expected growth from 10", stats.OrderbookBuffer.Capacity)
	}
	if stats.OrderbookBuffer.ResizeCount == 0 {
		t.Error("expected buffer to have resized")
	}
}

func TestRouter_InvalidJSON(t *testing.T) {
	input := make(chan connection.RawMessage, 10)
	cfg := DefaultRouterConfig()
	r := NewRouter(cfg, input, slog.Default())

	ctx := context.Background()
	if err := r.Start(ctx); err != nil {
		t.Fatalf("Start failed: %v", err)
	}
	defer r.Stop(ctx)

	// Send invalid JSON
	input <- connection.RawMessage{
		Data:       []byte(`{invalid json}`),
		ReceivedAt: time.Now(),
	}

	// Wait for processing
	time.Sleep(50 * time.Millisecond)

	stats := r.Stats()
	if stats.ParseErrors != 1 {
		t.Errorf("ParseErrors = %d, want 1", stats.ParseErrors)
	}
}

func TestRouter_UnknownMessageType(t *testing.T) {
	input := make(chan connection.RawMessage, 10)
	cfg := DefaultRouterConfig()
	r := NewRouter(cfg, input, slog.Default())

	ctx := context.Background()
	if err := r.Start(ctx); err != nil {
		t.Fatalf("Start failed: %v", err)
	}
	defer r.Stop(ctx)

	// Send unknown message type
	unknownMsg := map[string]interface{}{
		"type": "unknown_type",
		"msg":  map[string]interface{}{},
	}
	data, _ := json.Marshal(unknownMsg)

	input <- connection.RawMessage{
		Data:       data,
		ReceivedAt: time.Now(),
	}

	// Wait for processing
	time.Sleep(50 * time.Millisecond)

	stats := r.Stats()
	if stats.MessagesReceived != 1 {
		t.Errorf("MessagesReceived = %d, want 1", stats.MessagesReceived)
	}
	// Unknown types are skipped, not counted as errors
	if stats.MessagesRouted != 0 {
		t.Errorf("MessagesRouted = %d, want 0", stats.MessagesRouted)
	}
}

func TestRouter_ControlMessagesSkipped(t *testing.T) {
	input := make(chan connection.RawMessage, 10)
	cfg := DefaultRouterConfig()
	r := NewRouter(cfg, input, slog.Default())

	ctx := context.Background()
	if err := r.Start(ctx); err != nil {
		t.Fatalf("Start failed: %v", err)
	}
	defer r.Stop(ctx)

	// Send control messages that should be skipped
	controlTypes := []string{"subscribed", "unsubscribed", "error"}
	for _, ct := range controlTypes {
		msg := map[string]interface{}{
			"type": ct,
			"msg":  map[string]interface{}{},
		}
		data, _ := json.Marshal(msg)
		input <- connection.RawMessage{
			Data:       data,
			ReceivedAt: time.Now(),
		}
	}

	// Wait for processing
	time.Sleep(50 * time.Millisecond)

	stats := r.Stats()
	if stats.MessagesReceived != 3 {
		t.Errorf("MessagesReceived = %d, want 3", stats.MessagesReceived)
	}
	if stats.MessagesRouted != 0 {
		t.Errorf("MessagesRouted = %d, want 0 (control messages should be skipped)", stats.MessagesRouted)
	}
	if stats.ParseErrors != 0 {
		t.Errorf("ParseErrors = %d, want 0", stats.ParseErrors)
	}
}

func TestRouter_SubpennyPrice(t *testing.T) {
	input := make(chan connection.RawMessage, 10)
	cfg := DefaultRouterConfig()
	r := NewRouter(cfg, input, slog.Default())

	ctx := context.Background()
	if err := r.Start(ctx); err != nil {
		t.Fatalf("Start failed: %v", err)
	}
	defer r.Stop(ctx)

	// Send a delta with subpenny price
	deltaMsg := map[string]interface{}{
		"type": "orderbook_delta",
		"sid":  1,
		"seq":  1,
		"msg": map[string]interface{}{
			"market_ticker": "SUBPENNY",
			"price_dollars": "0.5250", // Subpenny price
			"delta":         10,
			"side":          "yes",
			"ts":            1705328200,
		},
	}
	data, _ := json.Marshal(deltaMsg)

	input <- connection.RawMessage{
		Data:       data,
		ReceivedAt: time.Now(),
	}

	time.Sleep(50 * time.Millisecond)

	buffers := r.Buffers()
	msg, ok := buffers.Orderbook.TryReceive()
	if !ok {
		t.Fatal("expected orderbook message")
	}

	// Router should pass through the price string unchanged
	if msg.PriceDollars != "0.5250" {
		t.Errorf("PriceDollars = %s, want 0.5250", msg.PriceDollars)
	}
}

func TestRouter_TimestampConversion(t *testing.T) {
	input := make(chan connection.RawMessage, 10)
	cfg := DefaultRouterConfig()
	r := NewRouter(cfg, input, slog.Default())

	ctx := context.Background()
	if err := r.Start(ctx); err != nil {
		t.Fatalf("Start failed: %v", err)
	}
	defer r.Stop(ctx)

	// Unix seconds timestamp
	unixSeconds := int64(1705328200)

	deltaMsg := map[string]interface{}{
		"type": "orderbook_delta",
		"sid":  1,
		"seq":  1,
		"msg": map[string]interface{}{
			"market_ticker": "TS-TEST",
			"price_dollars": "0.50",
			"delta":         1,
			"side":          "yes",
			"ts":            unixSeconds,
		},
	}
	data, _ := json.Marshal(deltaMsg)

	input <- connection.RawMessage{
		Data:       data,
		ReceivedAt: time.Now(),
	}

	time.Sleep(50 * time.Millisecond)

	buffers := r.Buffers()
	msg, ok := buffers.Orderbook.TryReceive()
	if !ok {
		t.Fatal("expected orderbook message")
	}

	expectedMicros := unixSeconds * 1_000_000
	if msg.ExchangeTs != expectedMicros {
		t.Errorf("ExchangeTs = %d, want %d (seconds * 1_000_000)", msg.ExchangeTs, expectedMicros)
	}
}

func TestRouter_SeqGapPassthrough(t *testing.T) {
	input := make(chan connection.RawMessage, 10)
	cfg := DefaultRouterConfig()
	r := NewRouter(cfg, input, slog.Default())

	ctx := context.Background()
	if err := r.Start(ctx); err != nil {
		t.Fatalf("Start failed: %v", err)
	}
	defer r.Stop(ctx)

	deltaMsg := map[string]interface{}{
		"type": "orderbook_delta",
		"sid":  1,
		"seq":  10,
		"msg": map[string]interface{}{
			"market_ticker": "GAP-TEST",
			"price_dollars": "0.50",
			"delta":         1,
			"side":          "yes",
			"ts":            1705328200,
		},
	}
	data, _ := json.Marshal(deltaMsg)

	input <- connection.RawMessage{
		Data:       data,
		ReceivedAt: time.Now(),
		SeqGap:     true,
		GapSize:    5,
	}

	time.Sleep(50 * time.Millisecond)

	buffers := r.Buffers()
	msg, ok := buffers.Orderbook.TryReceive()
	if !ok {
		t.Fatal("expected orderbook message")
	}

	if !msg.SeqGap {
		t.Error("SeqGap should be true")
	}
	if msg.GapSize != 5 {
		t.Errorf("GapSize = %d, want 5", msg.GapSize)
	}
}

func TestRouter_Stats(t *testing.T) {
	input := make(chan connection.RawMessage, 10)
	cfg := DefaultRouterConfig()
	r := NewRouter(cfg, input, slog.Default())

	ctx := context.Background()
	if err := r.Start(ctx); err != nil {
		t.Fatalf("Start failed: %v", err)
	}
	defer r.Stop(ctx)

	// Initial stats should be zero
	stats := r.Stats()
	if stats.MessagesReceived != 0 || stats.MessagesRouted != 0 {
		t.Error("initial stats should be zero")
	}

	// Send some messages
	for i := 0; i < 5; i++ {
		deltaMsg := map[string]interface{}{
			"type": "orderbook_delta",
			"sid":  1,
			"seq":  int64(i),
			"msg": map[string]interface{}{
				"market_ticker": "STATS-TEST",
				"price_dollars": "0.50",
				"delta":         1,
				"side":          "yes",
				"ts":            1705328200,
			},
		}
		data, _ := json.Marshal(deltaMsg)
		input <- connection.RawMessage{
			Data:       data,
			ReceivedAt: time.Now(),
		}
	}

	// Wait for processing
	time.Sleep(100 * time.Millisecond)

	stats = r.Stats()
	if stats.MessagesReceived != 5 {
		t.Errorf("MessagesReceived = %d, want 5", stats.MessagesReceived)
	}
	if stats.MessagesRouted != 5 {
		t.Errorf("MessagesRouted = %d, want 5", stats.MessagesRouted)
	}
}

func TestParsePriceLevels(t *testing.T) {
	tests := []struct {
		name  string
		input [][]interface{}
		want  []PriceLevel
	}{
		{
			name:  "empty",
			input: [][]interface{}{},
			want:  []PriceLevel{},
		},
		{
			name:  "single level",
			input: [][]interface{}{{"0.52", 100.0}},
			want:  []PriceLevel{{Dollars: "0.52", Quantity: 100}},
		},
		{
			name:  "multiple levels",
			input: [][]interface{}{{"0.52", 100.0}, {"0.51", 200.0}, {"0.50", 300.0}},
			want: []PriceLevel{
				{Dollars: "0.52", Quantity: 100},
				{Dollars: "0.51", Quantity: 200},
				{Dollars: "0.50", Quantity: 300},
			},
		},
		{
			name:  "subpenny price",
			input: [][]interface{}{{"0.5250", 50.0}},
			want:  []PriceLevel{{Dollars: "0.5250", Quantity: 50}},
		},
		{
			name:  "invalid level skipped",
			input: [][]interface{}{{"0.52"}, {"0.51", 200.0}}, // First level incomplete
			want:  []PriceLevel{{Dollars: "0.51", Quantity: 200}},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := parsePriceLevels(tt.input)
			if len(got) != len(tt.want) {
				t.Errorf("len = %d, want %d", len(got), len(tt.want))
				return
			}
			for i := range got {
				if got[i].Dollars != tt.want[i].Dollars {
					t.Errorf("[%d].Dollars = %s, want %s", i, got[i].Dollars, tt.want[i].Dollars)
				}
				if got[i].Quantity != tt.want[i].Quantity {
					t.Errorf("[%d].Quantity = %d, want %d", i, got[i].Quantity, tt.want[i].Quantity)
				}
			}
		})
	}
}
