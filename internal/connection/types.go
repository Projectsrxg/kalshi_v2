package connection

import (
	"encoding/json"
	"errors"
	"time"
)

// Errors
var (
	ErrNotConnected    = errors.New("not connected")
	ErrStaleConnection = errors.New("connection stale (no ping)")
	ErrTimeout         = errors.New("operation timeout")
	ErrAlreadyClosed   = errors.New("already closed")
)

// TimestampedMessage wraps raw message data with receive timestamp.
type TimestampedMessage struct {
	Data       []byte    // Raw message bytes from WebSocket
	ReceivedAt time.Time // Local timestamp when ReadMessage() returned
}

// RawMessage is a message from Connection Manager to Message Router.
type RawMessage struct {
	Data       []byte    // Raw message bytes from WebSocket
	ConnID     int       // Which connection this came from (1-150)
	ReceivedAt time.Time // Local timestamp when WS Client received message
	SeqGap     bool      // True if sequence gap detected before this message
	GapSize    int       // Number of missed messages (0 if no gap)
}

// Command is a WebSocket command to send to the server.
type Command struct {
	ID     int64       `json:"id"`
	Cmd    string      `json:"cmd"`
	Params interface{} `json:"params"`
}

// SubscribeParams are parameters for a subscribe command.
type SubscribeParams struct {
	Channels      []string `json:"channels"`
	MarketTicker  string   `json:"market_ticker,omitempty"`
	MarketTickers []string `json:"market_tickers,omitempty"`
}

// UnsubscribeParams are parameters for an unsubscribe command.
type UnsubscribeParams struct {
	SIDs []int64 `json:"sids"`
}

// UpdateSubscriptionParams are parameters for updating a subscription.
type UpdateSubscriptionParams struct {
	SIDs          []int64  `json:"sids"`
	Action        string   `json:"action"` // "add_markets" or "delete_markets"
	MarketTickers []string `json:"market_tickers"`
}

// Response is a command response from the server.
type Response struct {
	ID   int64           `json:"id"`
	Type string          `json:"type"` // "subscribed", "unsubscribed", "error", "ok"
	Msg  json.RawMessage `json:"msg"`
}

// SubscribedMsg is the message content for a "subscribed" response.
type SubscribedMsg struct {
	SID     int64  `json:"sid"`
	Channel string `json:"channel"`
}

// UnsubscribedMsg is the message content for an "unsubscribed" response.
type UnsubscribedMsg struct {
	SIDs []int64 `json:"sids"`
}

// ErrorMsg is the message content for an "error" response.
type ErrorMsg struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

// DataMessage is a data message from the server (ticker, trade, orderbook, lifecycle).
type DataMessage struct {
	Type string          `json:"type"` // "ticker", "trade", "orderbook_delta", "market_lifecycle"
	SID  int64           `json:"sid"`
	Seq  int64           `json:"seq,omitempty"` // Sequence number (orderbook only)
	Msg  json.RawMessage `json:"msg"`
}

// LifecycleMsg is the message content for a market_lifecycle message.
type LifecycleMsg struct {
	MarketTicker string `json:"market_ticker"`
	EventType    string `json:"event_type"` // "created", "status_change", "settled"
	OldStatus    string `json:"old_status"`
	NewStatus    string `json:"new_status"`
	Result       string `json:"result"` // "yes", "no", or ""
	Timestamp    int64  `json:"ts"`     // Unix timestamp (seconds)
}

// ClientConfig configures a WebSocket client.
type ClientConfig struct {
	URL          string        // WebSocket URL (e.g., wss://api.elections.kalshi.com/trade-api/ws/v2)
	KeyID        string        // API key ID for KALSHI-ACCESS-KEY header
	PrivateKey   interface{}   // *rsa.PrivateKey for signing (nil = no auth)
	PingTimeout  time.Duration // Max time without ping before considering connection stale
	WriteTimeout time.Duration // Write deadline for sends
	BufferSize   int           // Message channel buffer size
}

// DefaultClientConfig returns sensible defaults.
func DefaultClientConfig() ClientConfig {
	return ClientConfig{
		PingTimeout:  60 * time.Second,
		WriteTimeout: 5 * time.Second,
		BufferSize:   100000, // 100K per connection for high-volume markets
	}
}

// ManagerConfig configures the Connection Manager.
type ManagerConfig struct {
	WSURL             string        // WebSocket URL (e.g., wss://api.elections.kalshi.com/trade-api/ws/v2)
	KeyID             string        // API key ID for authentication
	PrivateKey        interface{}   // *rsa.PrivateKey for signing requests
	SubscribeTimeout  time.Duration // Timeout for subscribe commands
	ReconnectBaseWait time.Duration // Base wait time for reconnection
	ReconnectMaxWait  time.Duration // Max wait time for reconnection
	MessageBufferSize int           // Buffer size for output message channel
	WorkerCount       int           // Number of subscribe workers
}

// DefaultManagerConfig returns sensible defaults.
func DefaultManagerConfig() ManagerConfig {
	return ManagerConfig{
		SubscribeTimeout:  10 * time.Second,
		ReconnectBaseWait: 1 * time.Second,
		ReconnectMaxWait:  60 * time.Second,
		MessageBufferSize: 1000000, // 1M central buffer for 300K+ markets
		WorkerCount:       10,
	}
}

// ConnectionRole identifies the purpose of a connection.
type ConnectionRole string

const (
	RoleTicker    ConnectionRole = "ticker"
	RoleTrade     ConnectionRole = "trade"
	RoleLifecycle ConnectionRole = "lifecycle"
	RoleOrderbook ConnectionRole = "orderbook"
)

// Subscription tracks an active subscription.
type Subscription struct {
	SID     int64
	Channel string
	ConnID  int
	Ticker  string // Empty for global subscriptions
}
