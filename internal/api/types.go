package api

// ExchangeStatusResponse from GET /exchange/status
type ExchangeStatusResponse struct {
	ExchangeActive      bool   `json:"exchange_active"`
	TradingActive       bool   `json:"trading_active"`
	EstimatedResumeTime string `json:"exchange_estimated_resume_time,omitempty"`
}

// MarketsResponse from GET /markets
type MarketsResponse struct {
	Markets []APIMarket `json:"markets"`
	Cursor  string      `json:"cursor"`
}

// APIMarket represents a market from the Kalshi API.
type APIMarket struct {
	Ticker        string `json:"ticker"`
	EventTicker   string `json:"event_ticker"`
	Title         string `json:"title"`
	Subtitle      string `json:"subtitle"`
	Status        string `json:"status"`
	MarketType    string `json:"market_type"`
	Result        string `json:"result"`

	// Prices in cents
	YesBid    int `json:"yes_bid"`
	YesAsk    int `json:"yes_ask"`
	NoBid     int `json:"no_bid"`
	NoAsk     int `json:"no_ask"`
	LastPrice int `json:"last_price"`

	// Prices as strings (sub-penny)
	YesBidDollars   string `json:"yes_bid_dollars"`
	YesAskDollars   string `json:"yes_ask_dollars"`
	NoBidDollars    string `json:"no_bid_dollars"`
	NoAskDollars    string `json:"no_ask_dollars"`
	LastPriceDollars string `json:"last_price_dollars"`

	// Volume
	Volume       int64 `json:"volume"`
	Volume24h    int64 `json:"volume_24h"`
	OpenInterest int64 `json:"open_interest"`

	// Timestamps (ISO 8601)
	OpenTime       string `json:"open_time"`
	CloseTime      string `json:"close_time"`
	ExpirationTime string `json:"expiration_time"`
	CreatedTime    string `json:"created_time"`

	// Settlement
	SettlementValue        *int    `json:"settlement_value"`
	SettlementValueDollars *string `json:"settlement_value_dollars"`
}

// SingleMarketResponse from GET /markets/{ticker}
type SingleMarketResponse struct {
	Market APIMarket `json:"market"`
}

// EventsResponse from GET /events
type EventsResponse struct {
	Events []APIEvent `json:"events"`
	Cursor string     `json:"cursor"`
}

// APIEvent represents an event from the Kalshi API.
type APIEvent struct {
	EventTicker   string   `json:"event_ticker"`
	SeriesTicker  string   `json:"series_ticker"`
	Title         string   `json:"title"`
	Subtitle      string   `json:"subtitle"`
	Category      string   `json:"category"`
	Status        string   `json:"status"`
	MarketTickers []string `json:"markets"`
}

// SingleEventResponse from GET /events/{event_ticker}
type SingleEventResponse struct {
	Event APIEvent `json:"event"`
}

// SeriesResponse from GET /series/{series_ticker}
type SeriesResponse struct {
	Series APISeries `json:"series"`
}

// APISeries represents a series from the Kalshi API.
type APISeries struct {
	Ticker            string   `json:"ticker"`
	Title             string   `json:"title"`
	Category          string   `json:"category"`
	Frequency         string   `json:"frequency"`
	Tags              []string `json:"tags"`
	SettlementSources []string `json:"settlement_sources"`
}

// OrderbookResponse from GET /markets/{ticker}/orderbook
type OrderbookResponse struct {
	Orderbook APIOrderbook `json:"orderbook"`
}

// APIOrderbook represents the orderbook from the Kalshi API.
type APIOrderbook struct {
	// Levels as [price_cents, quantity] pairs
	Yes [][]int `json:"yes"`
	No  [][]int `json:"no"`
}

// GetMarketsOptions configures a GetMarkets request.
type GetMarketsOptions struct {
	Limit        int
	Cursor       string
	EventTicker  string
	SeriesTicker string
	Tickers      []string
	Status       string
}

// GetEventsOptions configures a GetEvents request.
type GetEventsOptions struct {
	Limit        int
	Cursor       string
	SeriesTicker string
	Status       string
}
