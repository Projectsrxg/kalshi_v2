package router

import (
	"encoding/json"
	"strconv"
	"time"
)

// FlexInt64 can unmarshal from either a JSON string or number.
// Kalshi sometimes sends timestamps as strings, sometimes as numbers.
type FlexInt64 int64

func (f *FlexInt64) UnmarshalJSON(data []byte) error {
	// Try as number first
	var i int64
	if err := json.Unmarshal(data, &i); err == nil {
		*f = FlexInt64(i)
		return nil
	}

	// Try as string
	var s string
	if err := json.Unmarshal(data, &s); err != nil {
		return err
	}

	// Try parsing as integer string first
	i, err := strconv.ParseInt(s, 10, 64)
	if err == nil {
		*f = FlexInt64(i)
		return nil
	}

	// Try parsing as ISO 8601 timestamp (e.g., "2026-01-06T15:24:59.504579Z")
	t, err := time.Parse(time.RFC3339Nano, s)
	if err != nil {
		return err
	}
	*f = FlexInt64(t.UnixMicro()) // Use microseconds per CLAUDE.md spec
	return nil
}

// RouterConfig holds configuration for the Message Router.
type RouterConfig struct {
	// Output buffer sizes
	OrderbookBufferSize int // Default: 5000
	TradeBufferSize     int // Default: 1000
	TickerBufferSize    int // Default: 1000
}

// DefaultRouterConfig returns default configuration.
func DefaultRouterConfig() RouterConfig {
	return RouterConfig{
		OrderbookBufferSize: 5000,
		TradeBufferSize:     1000,
		TickerBufferSize:    1000,
	}
}

// RouterChannels provides read-only access to output channels.
type RouterChannels struct {
	Orderbook <-chan OrderbookMsg
	Trade     <-chan TradeMsg
	Ticker    <-chan TickerMsg
}

// OrderbookMsg represents either a snapshot or delta message.
// Type field indicates which: "snapshot" or "delta".
type OrderbookMsg struct {
	Type string // "snapshot" or "delta"

	// Common fields
	Ticker     string
	SID        int64
	Seq        int64
	ReceivedAt time.Time
	SeqGap     bool
	GapSize    int

	// Snapshot-only fields (empty for delta)
	Yes []PriceLevel
	No  []PriceLevel

	// Delta-only fields (zero/empty for snapshot)
	PriceDollars string // e.g. "0.52" or "0.5250" for subpenny
	Delta        int
	Side         string // "yes" or "no"
	ExchangeTs   int64  // Microseconds
}

// PriceLevel represents a price point in an orderbook snapshot.
type PriceLevel struct {
	Dollars  string // e.g. "0.52", "0.5250" - Writer converts to internal format
	Quantity int
}

// TradeMsg represents a trade message from WebSocket.
type TradeMsg struct {
	Ticker          string
	TradeID         string
	Size            int    // Number of contracts (Kalshi: "count")
	YesPriceDollars string // e.g. "0.52"
	NoPriceDollars  string // e.g. "0.48"
	TakerSide       string // "yes" or "no"
	SID             int64
	Seq             int64
	ExchangeTs      int64 // Microseconds
	ReceivedAt      time.Time
	SeqGap          bool
	GapSize         int
}

// TickerMsg represents a ticker update message from WebSocket.
type TickerMsg struct {
	Ticker             string
	PriceDollars       string // Last price, e.g. "0.52"
	YesBidDollars      string
	YesAskDollars      string
	NoBidDollars       string
	Volume             int64
	OpenInterest       int64
	DollarVolume       int64
	DollarOpenInterest int64
	SID                int64
	ExchangeTs         int64 // Microseconds
	ReceivedAt         time.Time
	// Note: Ticker messages have no Seq field
}

// Wire types for JSON parsing

// orderbookSnapshotWire is the wire format for orderbook_snapshot messages.
type orderbookSnapshotWire struct {
	Type string `json:"type"`
	SID  int64  `json:"sid"`
	Seq  int64  `json:"seq"`
	Msg  struct {
		MarketTicker string          `json:"market_ticker"`
		YesDollars   [][]interface{} `json:"yes_dollars"` // [["0.52", qty], ...]
		NoDollars    [][]interface{} `json:"no_dollars"`
	} `json:"msg"`
}

// orderbookDeltaWire is the wire format for orderbook_delta messages.
type orderbookDeltaWire struct {
	Type string `json:"type"`
	SID  int64  `json:"sid"`
	Seq  int64  `json:"seq"`
	Msg  struct {
		MarketTicker string    `json:"market_ticker"`
		PriceDollars string    `json:"price_dollars"` // e.g. "0.52" or "0.5250"
		Delta        int       `json:"delta"`
		Side         string    `json:"side"`
		Ts           FlexInt64 `json:"ts"`
	} `json:"msg"`
}

// tradeWire is the wire format for trade messages.
type tradeWire struct {
	Type string `json:"type"`
	SID  int64  `json:"sid"`
	Seq  int64  `json:"seq"`
	Msg  struct {
		MarketTicker    string    `json:"market_ticker"`
		TradeID         string    `json:"trade_id"`
		Count           int       `json:"count"` // We store as "size"
		YesPriceDollars string    `json:"yes_price_dollars"`
		NoPriceDollars  string    `json:"no_price_dollars"`
		TakerSide       string    `json:"taker_side"`
		Ts              FlexInt64 `json:"ts"`
	} `json:"msg"`
}

// tickerWire is the wire format for ticker messages.
type tickerWire struct {
	Type string `json:"type"`
	SID  int64  `json:"sid"`
	Msg  struct {
		MarketTicker       string    `json:"market_ticker"`
		PriceDollars       string    `json:"price_dollars"`
		YesBidDollars      string    `json:"yes_bid_dollars"`
		YesAskDollars      string    `json:"yes_ask_dollars"`
		NoBidDollars       string    `json:"no_bid_dollars"`
		Volume             int64     `json:"volume"`
		OpenInterest       int64     `json:"open_interest"`
		DollarVolume       int64     `json:"dollar_volume"`
		DollarOpenInterest int64     `json:"dollar_open_interest"`
		Ts                 FlexInt64 `json:"ts"`
	} `json:"msg"`
}

// messageEnvelope is used for fast type extraction.
type messageEnvelope struct {
	Type string `json:"type"`
}
