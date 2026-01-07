package connection

import (
	"context"
	"log/slog"
	"net/http"
	"sync"
	"time"

	"github.com/gorilla/websocket"
	"github.com/rickgao/kalshi-data/internal/auth"
)

// Client represents a single WebSocket connection to Kalshi.
type Client interface {
	// Connect establishes the WebSocket connection.
	Connect(ctx context.Context) error

	// Close gracefully closes the connection.
	Close() error

	// Send writes raw bytes to the connection.
	Send(data []byte) error

	// Messages returns a channel of ALL raw messages (data + command responses).
	// Each message includes a local timestamp for when it was received.
	Messages() <-chan TimestampedMessage

	// Errors returns a channel of connection errors.
	Errors() <-chan error

	// IsConnected returns current connection state.
	IsConnected() bool
}

// client implements the Client interface.
type client struct {
	cfg    ClientConfig
	logger *slog.Logger

	conn *websocket.Conn

	// Output channels
	messages chan TimestampedMessage
	errors   chan error
	done     chan struct{}

	// Write serialization
	writeMu sync.Mutex

	// State
	mu         sync.RWMutex
	connected  bool
	lastPingAt time.Time
	closed     bool
}

// NewClient creates a new WebSocket client.
func NewClient(cfg ClientConfig, logger *slog.Logger) Client {
	if logger == nil {
		logger = slog.Default()
	}

	return &client{
		cfg:      cfg,
		logger:   logger,
		messages: make(chan TimestampedMessage, cfg.BufferSize),
		errors:   make(chan error, 1),
		done:     make(chan struct{}),
	}
}

// Connect establishes the WebSocket connection.
func (c *client) Connect(ctx context.Context) error {
	c.mu.Lock()
	if c.closed {
		c.mu.Unlock()
		return ErrAlreadyClosed
	}
	c.mu.Unlock()

	// Build headers with Kalshi authentication
	header := http.Header{}
	header.Set("Accept", "application/json")

	// Add RSA-PSS authentication headers if credentials are provided
	if c.cfg.KeyID != "" && c.cfg.PrivateKey != nil {
		creds := &auth.Credentials{
			KeyID:      c.cfg.KeyID,
			PrivateKey: c.cfg.PrivateKey,
		}

		authHeaders, err := creds.SignWebSocket()
		if err != nil {
			return err
		}

		for key, value := range authHeaders {
			header.Set(key, value)
		}
	}

	// Dial with context
	dialer := websocket.Dialer{
		HandshakeTimeout: 10 * time.Second,
	}

	conn, _, err := dialer.DialContext(ctx, c.cfg.URL, header)
	if err != nil {
		return err
	}

	c.mu.Lock()
	c.conn = conn
	c.connected = true
	c.lastPingAt = time.Now()
	c.mu.Unlock()

	// Set up ping handler - server sends ping, we respond with pong
	conn.SetPingHandler(func(data string) error {
		c.mu.Lock()
		c.lastPingAt = time.Now()
		c.mu.Unlock()

		c.writeMu.Lock()
		err := conn.WriteControl(
			websocket.PongMessage,
			[]byte(data),
			time.Now().Add(time.Second),
		)
		c.writeMu.Unlock()
		return err
	})

	// Set up pong handler - server responds to our ping
	conn.SetPongHandler(func(data string) error {
		c.mu.Lock()
		c.lastPingAt = time.Now()
		c.mu.Unlock()
		return nil
	})

	// Start goroutines
	go c.readLoop()
	go c.heartbeatLoop()

	c.logger.Debug("websocket connected", "url", c.cfg.URL)

	return nil
}

// Close gracefully closes the connection.
func (c *client) Close() error {
	c.mu.Lock()
	if c.closed {
		c.mu.Unlock()
		return nil
	}
	c.closed = true
	c.connected = false
	c.mu.Unlock()

	// Signal goroutines to stop
	close(c.done)

	// Close the WebSocket connection
	if c.conn != nil {
		// Send close message (protected by writeMu to prevent concurrent writes)
		c.writeMu.Lock()
		if err := c.conn.WriteControl(
			websocket.CloseMessage,
			websocket.FormatCloseMessage(websocket.CloseNormalClosure, ""),
			time.Now().Add(time.Second),
		); err != nil {
			c.logger.Debug("failed to send close message", "error", err)
		}
		c.writeMu.Unlock()
		return c.conn.Close()
	}

	return nil
}

// Send writes raw bytes to the connection.
func (c *client) Send(data []byte) error {
	c.mu.RLock()
	if !c.connected {
		c.mu.RUnlock()
		return ErrNotConnected
	}
	c.mu.RUnlock()

	c.writeMu.Lock()
	defer c.writeMu.Unlock()

	c.conn.SetWriteDeadline(time.Now().Add(c.cfg.WriteTimeout))
	return c.conn.WriteMessage(websocket.TextMessage, data)
}

// Messages returns the messages channel.
func (c *client) Messages() <-chan TimestampedMessage {
	return c.messages
}

// Errors returns the errors channel.
func (c *client) Errors() <-chan error {
	return c.errors
}

// IsConnected returns the current connection state.
func (c *client) IsConnected() bool {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.connected
}

// readLoop reads messages from the WebSocket and sends them to the messages channel.
func (c *client) readLoop() {
	defer func() {
		c.mu.Lock()
		c.connected = false
		c.mu.Unlock()
	}()

	for {
		select {
		case <-c.done:
			return
		default:
		}

		_, data, err := c.conn.ReadMessage()
		receivedAt := time.Now() // Capture timestamp immediately

		if err != nil {
			// Ignore errors after Close() is called
			select {
			case <-c.done:
				return
			default:
				select {
				case c.errors <- err:
				default:
					c.logger.Warn("error channel full, dropping error", "error", err)
				}
				return
			}
		}

		msg := TimestampedMessage{
			Data:       data,
			ReceivedAt: receivedAt,
		}

		select {
		case c.messages <- msg:
		case <-c.done:
			return
		default:
			c.logger.Error("message buffer full, dropping message (data loss)",
				"buffer_size", cap(c.messages),
				"msg_size", len(data),
			)
		}
	}
}

// heartbeatLoop monitors for stale connections and sends keepalive pings.
func (c *client) heartbeatLoop() {
	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-c.done:
			return
		case <-ticker.C:
			// Send a ping to keep connection alive
			c.mu.RLock()
			conn := c.conn
			c.mu.RUnlock()

			if conn != nil {
				c.writeMu.Lock()
				deadline := time.Now().Add(c.cfg.WriteTimeout)
				err := conn.WriteControl(websocket.PingMessage, []byte("keepalive"), deadline)
				c.writeMu.Unlock()
				if err != nil {
					c.logger.Warn("failed to send keepalive ping", "error", err)
				}
			}

			// Check for stale connection (no ping/pong activity)
			c.mu.RLock()
			lastPing := c.lastPingAt
			c.mu.RUnlock()

			if time.Since(lastPing) > c.cfg.PingTimeout {
				c.logger.Warn("connection stale, no ping/pong activity",
					"last_activity", lastPing,
					"timeout", c.cfg.PingTimeout,
				)
				select {
				case c.errors <- ErrStaleConnection:
				default:
					c.logger.Warn("error channel full, stale connection error dropped")
				}
				return
			}
		}
	}
}
