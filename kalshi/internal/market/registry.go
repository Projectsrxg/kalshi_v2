package market

import (
	"context"

	"github.com/rickgao/kalshi-data/internal/model"
)

// ChangeBufferSize is the capacity of the MarketChange channel.
const ChangeBufferSize = 1000

// Registry manages market discovery and lifecycle.
type Registry interface {
	// Start begins market discovery in background, returns immediately.
	// Emits MarketChange events as markets are discovered.
	Start(ctx context.Context) error

	// Stop gracefully shuts down.
	Stop(ctx context.Context) error

	// GetActiveMarkets returns all markets currently open for trading.
	GetActiveMarkets() []model.Market

	// GetMarket returns a specific market by ticker.
	GetMarket(ticker string) (model.Market, bool)

	// SubscribeChanges returns a channel of market state changes.
	// Connection Manager uses this to know when to subscribe/unsubscribe.
	SubscribeChanges() <-chan MarketChange

	// SetLifecycleSource sets the channel from which lifecycle messages are received.
	// Connection Manager calls this to provide market_lifecycle WebSocket messages.
	SetLifecycleSource(ch <-chan []byte)
}

// MarketChange represents a market state transition.
type MarketChange struct {
	Ticker    string        // Market ticker
	EventType string        // "created", "status_change", "settled"
	OldStatus string        // Previous status (for status_change)
	NewStatus string        // New status
	Market    *model.Market // Full market data (nil for "settled")
}
