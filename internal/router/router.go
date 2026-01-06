package router

import (
	"context"
	"encoding/json"
	"log/slog"
	"sync"

	"github.com/rickgao/kalshi-data/internal/connection"
)

// Router parses raw WebSocket messages and routes them to specialized Writers.
type Router interface {
	// Start begins routing messages from input channel to writers.
	Start(ctx context.Context) error

	// Stop gracefully shuts down the router.
	Stop(ctx context.Context) error

	// Buffers returns output buffers for writers to consume.
	Buffers() RouterBuffers

	// Stats returns current router statistics.
	Stats() RouterStats
}

// RouterBuffers provides access to output buffers for writers.
type RouterBuffers struct {
	Orderbook *GrowableBuffer[OrderbookMsg]
	Trade     *GrowableBuffer[TradeMsg]
	Ticker    *GrowableBuffer[TickerMsg]
}

// RouterStats contains runtime statistics.
type RouterStats struct {
	MessagesReceived int64
	MessagesRouted   int64
	ParseErrors      int64
	UnknownMessages  int64
	OrderbookBuffer  BufferStats
	TradeBuffer      BufferStats
	TickerBuffer     BufferStats
}

// router is the internal implementation.
type router struct {
	cfg    RouterConfig
	logger *slog.Logger

	// Input from Connection Manager
	input <-chan connection.RawMessage

	// Output to Writers (growable buffers)
	orderbookBuf *GrowableBuffer[OrderbookMsg]
	tradeBuf     *GrowableBuffer[TradeMsg]
	tickerBuf    *GrowableBuffer[TickerMsg]

	// Lifecycle
	ctx    context.Context
	cancel context.CancelFunc
	wg     sync.WaitGroup

	// Stats (atomic operations for thread safety)
	mu              sync.RWMutex
	received        int64
	routed          int64
	parseErrors     int64
	unknownMessages int64
}

// NewRouter creates a new Message Router.
func NewRouter(cfg RouterConfig, input <-chan connection.RawMessage, logger *slog.Logger) Router {
	if logger == nil {
		logger = slog.Default()
	}

	return &router{
		cfg:          cfg,
		logger:       logger,
		input:        input,
		orderbookBuf: NewGrowableBuffer[OrderbookMsg](cfg.OrderbookBufferSize),
		tradeBuf:     NewGrowableBuffer[TradeMsg](cfg.TradeBufferSize),
		tickerBuf:    NewGrowableBuffer[TickerMsg](cfg.TickerBufferSize),
	}
}

// Start begins routing messages.
func (r *router) Start(ctx context.Context) error {
	r.ctx, r.cancel = context.WithCancel(ctx)

	r.wg.Add(1)
	go r.routeLoop()

	r.logger.Info("message router started",
		"orderbook_buffer", r.cfg.OrderbookBufferSize,
		"trade_buffer", r.cfg.TradeBufferSize,
		"ticker_buffer", r.cfg.TickerBufferSize,
	)

	return nil
}

// Stop gracefully shuts down the router.
func (r *router) Stop(ctx context.Context) error {
	r.logger.Info("stopping message router")

	if r.cancel != nil {
		r.cancel()
	}

	// Wait for goroutine to finish
	done := make(chan struct{})
	go func() {
		r.wg.Wait()
		close(done)
	}()

	select {
	case <-done:
		r.logger.Info("message router stopped")
	case <-ctx.Done():
		r.logger.Warn("message router stop timed out")
	}

	// Close output buffers
	r.orderbookBuf.Close()
	r.tradeBuf.Close()
	r.tickerBuf.Close()

	return nil
}

// Buffers returns output buffers for writers.
func (r *router) Buffers() RouterBuffers {
	return RouterBuffers{
		Orderbook: r.orderbookBuf,
		Trade:     r.tradeBuf,
		Ticker:    r.tickerBuf,
	}
}

// Stats returns current statistics.
func (r *router) Stats() RouterStats {
	r.mu.RLock()
	defer r.mu.RUnlock()

	return RouterStats{
		MessagesReceived: r.received,
		MessagesRouted:   r.routed,
		ParseErrors:      r.parseErrors,
		UnknownMessages:  r.unknownMessages,
		OrderbookBuffer:  r.orderbookBuf.Stats(),
		TradeBuffer:      r.tradeBuf.Stats(),
		TickerBuffer:     r.tickerBuf.Stats(),
	}
}

// routeLoop is the main routing goroutine.
func (r *router) routeLoop() {
	defer r.wg.Done()

	for {
		select {
		case <-r.ctx.Done():
			return
		case raw, ok := <-r.input:
			if !ok {
				r.logger.Info("input channel closed")
				return
			}
			r.route(raw)
		}
	}
}

// route parses and routes a single message.
func (r *router) route(raw connection.RawMessage) {
	r.mu.Lock()
	r.received++
	r.mu.Unlock()

	// Extract message type
	msgType, err := r.extractType(raw.Data)
	if err != nil {
		r.logger.Warn("failed to extract message type", "error", err)
		r.mu.Lock()
		r.parseErrors++
		r.mu.Unlock()
		return
	}

	var sent bool

	switch msgType {
	case "orderbook_snapshot":
		msg, err := r.parseOrderbookSnapshot(raw)
		if err != nil {
			r.logger.Warn("failed to parse orderbook snapshot", "error", err)
			r.mu.Lock()
			r.parseErrors++
			r.mu.Unlock()
			return
		}
		sent = r.orderbookBuf.Send(msg)

	case "orderbook_delta":
		msg, err := r.parseOrderbookDelta(raw)
		if err != nil {
			r.logger.Warn("failed to parse orderbook delta", "error", err)
			r.mu.Lock()
			r.parseErrors++
			r.mu.Unlock()
			return
		}
		sent = r.orderbookBuf.Send(msg)

	case "trade":
		msg, err := r.parseTrade(raw)
		if err != nil {
			r.logger.Warn("failed to parse trade", "error", err)
			r.mu.Lock()
			r.parseErrors++
			r.mu.Unlock()
			return
		}
		sent = r.tradeBuf.Send(msg)

	case "ticker":
		msg, err := r.parseTicker(raw)
		if err != nil {
			r.logger.Warn("failed to parse ticker", "error", err)
			r.mu.Lock()
			r.parseErrors++
			r.mu.Unlock()
			return
		}
		sent = r.tickerBuf.Send(msg)

	default:
		// Skip control messages like "subscribed", "unsubscribed", "error"
		if msgType != "subscribed" && msgType != "unsubscribed" && msgType != "error" {
			r.logger.Debug("skipping message type", "type", msgType)
		}
		return
	}

	if sent {
		r.mu.Lock()
		r.routed++
		r.mu.Unlock()
	}
}

// extractType extracts the message type without full JSON parse.
func (r *router) extractType(data []byte) (string, error) {
	var envelope messageEnvelope
	if err := json.Unmarshal(data, &envelope); err != nil {
		return "", err
	}
	return envelope.Type, nil
}

// parseOrderbookSnapshot parses an orderbook_snapshot message.
func (r *router) parseOrderbookSnapshot(raw connection.RawMessage) (OrderbookMsg, error) {
	var wire orderbookSnapshotWire
	if err := json.Unmarshal(raw.Data, &wire); err != nil {
		return OrderbookMsg{}, err
	}

	return OrderbookMsg{
		Type:       "snapshot",
		Ticker:     wire.Msg.MarketTicker,
		SID:        wire.SID,
		Seq:        wire.Seq,
		ReceivedAt: raw.ReceivedAt,
		SeqGap:     raw.SeqGap,
		GapSize:    raw.GapSize,
		Yes:        parsePriceLevels(wire.Msg.YesDollars),
		No:         parsePriceLevels(wire.Msg.NoDollars),
	}, nil
}

// parseOrderbookDelta parses an orderbook_delta message.
func (r *router) parseOrderbookDelta(raw connection.RawMessage) (OrderbookMsg, error) {
	var wire orderbookDeltaWire
	if err := json.Unmarshal(raw.Data, &wire); err != nil {
		return OrderbookMsg{}, err
	}

	return OrderbookMsg{
		Type:         "delta",
		Ticker:       wire.Msg.MarketTicker,
		SID:          wire.SID,
		Seq:          wire.Seq,
		ReceivedAt:   raw.ReceivedAt,
		SeqGap:       raw.SeqGap,
		GapSize:      raw.GapSize,
		PriceDollars: wire.Msg.PriceDollars,
		Delta:        wire.Msg.Delta,
		Side:         wire.Msg.Side,
		ExchangeTs:   int64(wire.Msg.Ts) * 1_000_000, // seconds → microseconds
	}, nil
}

// parseTrade parses a trade message.
func (r *router) parseTrade(raw connection.RawMessage) (TradeMsg, error) {
	var wire tradeWire
	if err := json.Unmarshal(raw.Data, &wire); err != nil {
		return TradeMsg{}, err
	}

	return TradeMsg{
		Ticker:          wire.Msg.MarketTicker,
		TradeID:         wire.Msg.TradeID,
		Size:            wire.Msg.Count, // Kalshi: "count" → internal: "size"
		YesPriceDollars: wire.Msg.YesPriceDollars,
		NoPriceDollars:  wire.Msg.NoPriceDollars,
		TakerSide:       wire.Msg.TakerSide,
		SID:             wire.SID,
		Seq:             wire.Seq,
		ExchangeTs:      int64(wire.Msg.Ts) * 1_000_000,
		ReceivedAt:      raw.ReceivedAt,
		SeqGap:          raw.SeqGap,
		GapSize:         raw.GapSize,
	}, nil
}

// parseTicker parses a ticker message.
func (r *router) parseTicker(raw connection.RawMessage) (TickerMsg, error) {
	var wire tickerWire
	if err := json.Unmarshal(raw.Data, &wire); err != nil {
		return TickerMsg{}, err
	}

	return TickerMsg{
		Ticker:             wire.Msg.MarketTicker,
		PriceDollars:       wire.Msg.PriceDollars,
		YesBidDollars:      wire.Msg.YesBidDollars,
		YesAskDollars:      wire.Msg.YesAskDollars,
		NoBidDollars:       wire.Msg.NoBidDollars,
		Volume:             wire.Msg.Volume,
		OpenInterest:       wire.Msg.OpenInterest,
		DollarVolume:       wire.Msg.DollarVolume,
		DollarOpenInterest: wire.Msg.DollarOpenInterest,
		SID:                wire.SID,
		ExchangeTs:         int64(wire.Msg.Ts) * 1_000_000,
		ReceivedAt:         raw.ReceivedAt,
		// Note: ticker has no Seq field
	}, nil
}

// parsePriceLevels converts [["0.52", 100], ["0.51", 200]] to []PriceLevel.
func parsePriceLevels(levels [][]interface{}) []PriceLevel {
	result := make([]PriceLevel, 0, len(levels))
	for _, level := range levels {
		if len(level) < 2 {
			continue
		}
		dollars, _ := level[0].(string)
		qty, _ := level[1].(float64)
		result = append(result, PriceLevel{
			Dollars:  dollars,
			Quantity: int(qty),
		})
	}
	return result
}
