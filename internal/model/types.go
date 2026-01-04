package model

import "github.com/google/uuid"

// -----------------------------------------------------------------------------
// Relational Types
// -----------------------------------------------------------------------------

// Series represents a collection of related events (e.g., "US Presidential Election").
type Series struct {
	Ticker            string            // Primary key (e.g., "PRES")
	Title             string            // Display title
	Category          string            // Category (e.g., "Politics")
	Frequency         string            // Update frequency
	Tags              map[string]string // Arbitrary tags
	SettlementSources []string          // Data sources for settlement
	UpdatedAt         int64             // Last update (µs since epoch)
}

// Event represents a specific event within a series (e.g., "2024 Presidential Election").
type Event struct {
	EventTicker  string // Primary key (e.g., "PRES-2024")
	SeriesTicker string // Foreign key to Series
	Title        string // Display title
	Category     string // Category
	SubTitle     string // Optional subtitle
	CreatedTS    int64  // Creation time (µs since epoch)
	UpdatedAt    int64  // Last update (µs since epoch)
}

// Market represents a tradeable prediction market.
type Market struct {
	Ticker        string // Primary key (e.g., "PRES-2024-DEM")
	EventTicker   string // Foreign key to Event
	Title         string // Display title
	Subtitle      string // Optional subtitle
	MarketStatus  string // Status: initialized, inactive, active, closed, determined, disputed, amended, finalized
	TradingStatus string // Trading status
	MarketType    string // "binary" or "scalar"
	Result        string // Settlement result (yes/no/null)

	// Current prices (hundred-thousandths, 0-100,000)
	YesBid    int // Best YES bid price
	YesAsk    int // Best YES ask price
	LastPrice int // Last traded price

	// Volume
	Volume       int64 // Total volume
	Volume24h    int64 // 24-hour volume
	OpenInterest int64 // Open interest

	// Timing (µs since epoch)
	OpenTS       int64 // Market open time
	CloseTS      int64 // Market close time
	ExpirationTS int64 // Expiration time
	CreatedTS    int64 // Creation time
	UpdatedAt    int64 // Last update
}

// -----------------------------------------------------------------------------
// Time-Series Types
// -----------------------------------------------------------------------------

// Trade represents an executed trade.
type Trade struct {
	TradeID    uuid.UUID // Primary key (from Kalshi)
	ExchangeTS int64     // Kalshi server timestamp (µs since epoch)
	ReceivedAt int64     // Gatherer receive timestamp (µs since epoch)
	Ticker     string    // Market ticker
	Price      int       // Trade price (hundred-thousandths, 0-100,000)
	Size       int       // Number of contracts
	TakerSide  bool      // true = YES taker, false = NO taker
}

// OrderbookDelta represents a change to the orderbook at a specific price level.
type OrderbookDelta struct {
	ExchangeTS int64  // Kalshi server timestamp (µs since epoch)
	ReceivedAt int64  // Gatherer receive timestamp (µs since epoch)
	Ticker     string // Market ticker
	Side       bool   // true = YES, false = NO
	Price      int    // Price level (hundred-thousandths, 0-100,000)
	SizeDelta  int    // Change in size (positive = add, negative = remove)
	Seq        int64  // Kalshi sequence number (per-subscription)
}

// PriceLevel represents a single price level in an orderbook.
type PriceLevel struct {
	Price int // Price (hundred-thousandths, 0-100,000)
	Size  int // Quantity at this price
}

// OrderbookSnapshot represents a full orderbook state at a point in time.
type OrderbookSnapshot struct {
	SnapshotTS int64        // Snapshot timestamp (µs since epoch)
	ExchangeTS int64        // Kalshi server timestamp (µs since epoch), 0 if not provided
	Ticker     string       // Market ticker
	Source     string       // "ws" or "rest"
	YesBids    []PriceLevel // YES side bids (buy orders)
	YesAsks    []PriceLevel // YES side asks (sell orders)
	NoBids     []PriceLevel // NO side bids
	NoAsks     []PriceLevel // NO side asks
	BestYesBid int          // Best YES bid price
	BestYesAsk int          // Best YES ask price
	Spread     int          // Spread (BestYesAsk - BestYesBid)
}

// Ticker represents a market ticker update (price/volume snapshot).
type Ticker struct {
	ExchangeTS         int64  // Kalshi server timestamp (µs since epoch)
	ReceivedAt         int64  // Gatherer receive timestamp (µs since epoch)
	Ticker             string // Market ticker
	YesBid             int    // Best YES bid (hundred-thousandths)
	YesAsk             int    // Best YES ask (hundred-thousandths)
	LastPrice          int    // Last trade price (hundred-thousandths)
	Volume             int64  // Total volume
	OpenInterest       int64  // Open interest
	DollarVolume       int64  // Dollar-denominated volume
	DollarOpenInterest int64  // Dollar-denominated open interest
}
