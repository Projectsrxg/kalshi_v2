package connection

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"math"
	"sync"
	"sync/atomic"
	"time"

	"github.com/rickgao/kalshi-data/internal/market"
)

// Manager orchestrates WebSocket connections and subscriptions.
type Manager interface {
	// Start begins listening for Market Registry events and manages connections.
	Start(ctx context.Context) error

	// Stop gracefully shuts down all connections.
	Stop(ctx context.Context) error

	// Messages returns channel of raw messages for Message Router.
	Messages() <-chan RawMessage

	// Stats returns current connection and subscription statistics.
	Stats() ManagerStats
}

// ManagerStats provides statistics about the connection manager.
type ManagerStats struct {
	ConnectedCount     int
	TotalSubscriptions int
	MarketsSubscribed  int
}

// connState holds the state for a single connection.
type connState struct {
	client Client
	id     int            // Connection ID (1-150)
	role   ConnectionRole // "ticker", "trade", "lifecycle", "orderbook"

	// Markets on this connection (orderbook only)
	mu      sync.Mutex
	markets map[string]struct{}

	// Goroutine coordination
	readLoopDone chan struct{}

	// Command/response correlation
	pendingMu sync.Mutex
	pending   map[int64]chan Response
	cmdID     int64 // Atomic counter
}

// manager implements the Manager interface.
type manager struct {
	cfg      ManagerConfig
	registry market.Registry
	logger   *slog.Logger

	// Output channels
	router    chan RawMessage // Output to Message Router
	lifecycle chan []byte     // Output to Market Registry (market_lifecycle messages)

	ctx    context.Context
	cancel context.CancelFunc
	wg     sync.WaitGroup

	// Fixed connection pool (150 connections)
	tickerConns    [2]*connState   // Connections 1-2
	tradeConns     [2]*connState   // Connections 3-4
	lifecycleConns [2]*connState   // Connections 5-6
	orderbookConns [144]*connState // Connections 7-150

	// Market → connection mapping (for orderbook)
	marketConnMu sync.RWMutex
	marketToConn map[string]int // market ticker → connection ID (7-150)

	// Subscription tracking
	subsMu sync.RWMutex
	subs   map[int64]*Subscription // SID → subscription info

	// Sequence tracking (per connection+SID, since SIDs can collide across connections)
	seqMu   sync.RWMutex
	lastSeq map[seqKey]int64
}

// seqKey uniquely identifies a sequence stream (connection + subscription).
// SIDs are unique per-connection, not globally, so we need both.
type seqKey struct {
	connID int
	sid    int64
}

// NewManager creates a new Connection Manager.
func NewManager(cfg ManagerConfig, registry market.Registry, logger *slog.Logger) Manager {
	if logger == nil {
		logger = slog.Default()
	}

	return &manager{
		cfg:          cfg,
		registry:     registry,
		logger:       logger,
		router:       make(chan RawMessage, cfg.MessageBufferSize),
		lifecycle:    make(chan []byte, 100),
		marketToConn: make(map[string]int),
		subs:         make(map[int64]*Subscription),
		lastSeq:      make(map[seqKey]int64),
	}
}

// Start begins the connection manager.
func (m *manager) Start(ctx context.Context) error {
	m.ctx, m.cancel = context.WithCancel(ctx)

	// Initialize all connections
	if err := m.initConnections(); err != nil {
		return fmt.Errorf("init connections: %w", err)
	}

	// Start read loops for all connections
	m.startReadLoops()

	// Subscribe global channels
	if err := m.subscribeGlobalChannels(); err != nil {
		m.logger.Error("failed to subscribe global channels", "error", err)
		// Continue anyway - will retry on reconnection
	}

	// Set lifecycle source for Market Registry
	m.registry.SetLifecycleSource(m.lifecycle)

	// Start market change handler
	m.wg.Add(1)
	go m.handleMarketChanges()

	// Subscribe to existing active markets
	m.subscribeExistingMarkets()

	m.logger.Info("connection manager started",
		"orderbook_conns", len(m.orderbookConns),
		"global_conns", 6,
	)

	return nil
}

// Stop gracefully shuts down.
func (m *manager) Stop(ctx context.Context) error {
	m.logger.Info("stopping connection manager")

	if m.cancel != nil {
		m.cancel()
	}

	// Wait for goroutines with timeout
	done := make(chan struct{})
	go func() {
		m.wg.Wait()
		close(done)
	}()

	select {
	case <-done:
	case <-ctx.Done():
		m.logger.Warn("shutdown timeout, forcing close")
	}

	// Close all connections
	m.closeAllConnections()

	close(m.router)
	close(m.lifecycle)

	m.logger.Info("connection manager stopped")
	return nil
}

// Messages returns the output channel for Message Router.
func (m *manager) Messages() <-chan RawMessage {
	return m.router
}

// Stats returns current statistics.
func (m *manager) Stats() ManagerStats {
	connected := 0
	for _, c := range m.tickerConns {
		if c != nil && c.client.IsConnected() {
			connected++
		}
	}
	for _, c := range m.tradeConns {
		if c != nil && c.client.IsConnected() {
			connected++
		}
	}
	for _, c := range m.lifecycleConns {
		if c != nil && c.client.IsConnected() {
			connected++
		}
	}
	for _, c := range m.orderbookConns {
		if c != nil && c.client.IsConnected() {
			connected++
		}
	}

	m.subsMu.RLock()
	totalSubs := len(m.subs)
	m.subsMu.RUnlock()

	m.marketConnMu.RLock()
	marketsSubbed := len(m.marketToConn)
	m.marketConnMu.RUnlock()

	return ManagerStats{
		ConnectedCount:     connected,
		TotalSubscriptions: totalSubs,
		MarketsSubscribed:  marketsSubbed,
	}
}

// initConnections creates all WebSocket connections.
// Returns error if critical connections (lifecycle) all fail.
func (m *manager) initConnections() error {
	clientCfg := ClientConfig{
		URL:          m.cfg.WSURL,
		KeyID:        m.cfg.KeyID,
		PrivateKey:   m.cfg.PrivateKey,
		PingTimeout:  5 * time.Minute, // Long timeout to survive initial snapshot flood
		WriteTimeout: 5 * time.Second,
		BufferSize:   100000, // 100K per connection for high-volume initial snapshots
	}

	var tickerConnected, tradeConnected, lifecycleConnected, orderbookConnected int

	// Ticker connections (1-2)
	for i := 0; i < 2; i++ {
		conn := m.newConnState(i+1, RoleTicker, clientCfg)
		if err := conn.client.Connect(m.ctx); err != nil {
			m.logger.Warn("failed to connect ticker", "id", i+1, "error", err)
		} else {
			tickerConnected++
		}
		m.tickerConns[i] = conn
	}

	// Trade connections (3-4)
	for i := 0; i < 2; i++ {
		conn := m.newConnState(i+3, RoleTrade, clientCfg)
		if err := conn.client.Connect(m.ctx); err != nil {
			m.logger.Warn("failed to connect trade", "id", i+3, "error", err)
		} else {
			tradeConnected++
		}
		m.tradeConns[i] = conn
	}

	// Lifecycle connections (5-6)
	for i := 0; i < 2; i++ {
		conn := m.newConnState(i+5, RoleLifecycle, clientCfg)
		if err := conn.client.Connect(m.ctx); err != nil {
			m.logger.Warn("failed to connect lifecycle", "id", i+5, "error", err)
		} else {
			lifecycleConnected++
		}
		m.lifecycleConns[i] = conn
	}

	// Orderbook connections (7-150)
	for i := 0; i < 144; i++ {
		conn := m.newConnState(i+7, RoleOrderbook, clientCfg)
		if err := conn.client.Connect(m.ctx); err != nil {
			m.logger.Warn("failed to connect orderbook", "id", i+7, "error", err)
		} else {
			orderbookConnected++
		}
		m.orderbookConns[i] = conn
	}

	m.logger.Info("initial connections established",
		"ticker", tickerConnected,
		"trade", tradeConnected,
		"lifecycle", lifecycleConnected,
		"orderbook", orderbookConnected,
	)

	// Fail if no lifecycle connections - we need market updates
	if lifecycleConnected == 0 {
		return fmt.Errorf("failed to establish any lifecycle connections")
	}

	// Warn if no orderbook connections
	if orderbookConnected == 0 {
		m.logger.Error("no orderbook connections established - data collection will fail")
	}

	return nil
}

// newConnState creates a new connection state.
func (m *manager) newConnState(id int, role ConnectionRole, cfg ClientConfig) *connState {
	return &connState{
		client:       NewClient(cfg, m.logger.With("conn_id", id, "role", role)),
		id:           id,
		role:         role,
		markets:      make(map[string]struct{}),
		readLoopDone: make(chan struct{}),
		pending:      make(map[int64]chan Response),
	}
}

// startReadLoops starts read loops for all connections.
func (m *manager) startReadLoops() {
	for _, c := range m.tickerConns {
		if c != nil {
			m.wg.Add(1)
			go m.readLoop(c)
		}
	}
	for _, c := range m.tradeConns {
		if c != nil {
			m.wg.Add(1)
			go m.readLoop(c)
		}
	}
	for _, c := range m.lifecycleConns {
		if c != nil {
			m.wg.Add(1)
			go m.readLoop(c)
		}
	}
	for _, c := range m.orderbookConns {
		if c != nil {
			m.wg.Add(1)
			go m.readLoop(c)
		}
	}
}

// subscribeGlobalChannels subscribes to ticker, trade, and lifecycle channels.
// Returns error if no lifecycle subscriptions succeed (critical for market updates).
func (m *manager) subscribeGlobalChannels() error {
	var tickerSubs, tradeSubs, lifecycleSubs int

	// Subscribe ticker on both connections
	for i, c := range m.tickerConns {
		if c == nil || !c.client.IsConnected() {
			continue
		}
		if err := m.subscribe(c, "ticker", ""); err != nil {
			m.logger.Error("failed to subscribe ticker", "conn", i+1, "error", err)
		} else {
			tickerSubs++
		}
	}

	// Subscribe trade on both connections
	for i, c := range m.tradeConns {
		if c == nil || !c.client.IsConnected() {
			continue
		}
		if err := m.subscribe(c, "trade", ""); err != nil {
			m.logger.Error("failed to subscribe trade", "conn", i+3, "error", err)
		} else {
			tradeSubs++
		}
	}

	// Subscribe lifecycle on both connections
	for i, c := range m.lifecycleConns {
		if c == nil || !c.client.IsConnected() {
			continue
		}
		if err := m.subscribe(c, "market_lifecycle_v2", ""); err != nil {
			m.logger.Error("failed to subscribe lifecycle", "conn", i+5, "error", err)
		} else {
			lifecycleSubs++
		}
	}

	m.logger.Info("global channel subscriptions",
		"ticker", tickerSubs,
		"trade", tradeSubs,
		"lifecycle", lifecycleSubs,
	)

	// Fail if no lifecycle subscriptions - we need market updates
	if lifecycleSubs == 0 {
		return fmt.Errorf("failed to subscribe to any lifecycle channels")
	}

	return nil
}

// subscribeExistingMarkets subscribes to orderbooks for all active markets.
func (m *manager) subscribeExistingMarkets() {
	markets := m.registry.GetActiveMarkets()
	m.logger.Info("subscribing to existing markets", "count", len(markets))

	for _, mkt := range markets {
		m.subscribeOrderbook(mkt.Ticker)
	}
}

// closeAllConnections closes all WebSocket connections.
func (m *manager) closeAllConnections() {
	for _, c := range m.tickerConns {
		if c != nil {
			c.client.Close()
		}
	}
	for _, c := range m.tradeConns {
		if c != nil {
			c.client.Close()
		}
	}
	for _, c := range m.lifecycleConns {
		if c != nil {
			c.client.Close()
		}
	}
	for _, c := range m.orderbookConns {
		if c != nil {
			c.client.Close()
		}
	}
}

// handleMarketChanges processes market change events from the registry.
func (m *manager) handleMarketChanges() {
	defer m.wg.Done()

	changes := m.registry.SubscribeChanges()

	// Worker pool for non-blocking subscribes
	workCh := make(chan market.MarketChange, 100)
	for i := 0; i < m.cfg.WorkerCount; i++ {
		m.wg.Add(1)
		go m.subscribeWorker(workCh)
	}

	for {
		select {
		case <-m.ctx.Done():
			close(workCh)
			return
		case change, ok := <-changes:
			if !ok {
				close(workCh)
				return
			}
			select {
			case workCh <- change:
			default:
				m.logger.Warn("subscribe worker backpressure, dropping event",
					"ticker", change.Ticker,
				)
			}
		}
	}
}

// subscribeWorker processes market changes from the work queue.
func (m *manager) subscribeWorker(workCh <-chan market.MarketChange) {
	defer m.wg.Done()

	for change := range workCh {
		m.handleMarketChange(change)
	}
}

// handleMarketChange processes a single market change event.
func (m *manager) handleMarketChange(change market.MarketChange) {
	switch change.EventType {
	case "created":
		if isActiveStatus(change.NewStatus) {
			m.subscribeOrderbook(change.Ticker)
		}

	case "status_change":
		wasActive := isActiveStatus(change.OldStatus)
		isActive := isActiveStatus(change.NewStatus)

		if isActive && !wasActive {
			m.subscribeOrderbook(change.Ticker)
		} else if !isActive && wasActive {
			m.unsubscribeOrderbook(change.Ticker)
		}

	case "settled":
		m.unsubscribeOrderbook(change.Ticker)
	}
}

// isActiveStatus returns true if the status indicates an active market.
func isActiveStatus(status string) bool {
	return status == "open" || status == "active"
}

// selectOrderbookConn returns the orderbook connection with fewest subscriptions.
func (m *manager) selectOrderbookConn() *connState {
	var minConn *connState
	minCount := math.MaxInt

	for _, conn := range m.orderbookConns {
		if conn == nil || !conn.client.IsConnected() {
			continue
		}
		conn.mu.Lock()
		count := len(conn.markets)
		conn.mu.Unlock()

		if count < minCount {
			minCount = count
			minConn = conn
		}
	}

	return minConn
}

// subscribeOrderbook subscribes to orderbook updates for a market.
func (m *manager) subscribeOrderbook(ticker string) {
	// Check if already subscribed
	m.marketConnMu.RLock()
	_, exists := m.marketToConn[ticker]
	m.marketConnMu.RUnlock()
	if exists {
		return
	}

	conn := m.selectOrderbookConn()
	if conn == nil {
		m.logger.Error("no healthy orderbook connections", "ticker", ticker)
		return
	}

	// Track assignment
	m.marketConnMu.Lock()
	m.marketToConn[ticker] = conn.id
	m.marketConnMu.Unlock()

	conn.mu.Lock()
	conn.markets[ticker] = struct{}{}
	conn.mu.Unlock()

	// Send subscribe command
	if err := m.subscribe(conn, "orderbook_delta", ticker); err != nil {
		m.logger.Warn("failed to subscribe orderbook",
			"ticker", ticker,
			"conn", conn.id,
			"error", err,
		)

		// Rollback tracking
		conn.mu.Lock()
		delete(conn.markets, ticker)
		conn.mu.Unlock()

		m.marketConnMu.Lock()
		delete(m.marketToConn, ticker)
		m.marketConnMu.Unlock()
	}
}

// unsubscribeOrderbook unsubscribes from orderbook updates for a market.
func (m *manager) unsubscribeOrderbook(ticker string) {
	m.marketConnMu.RLock()
	connID, ok := m.marketToConn[ticker]
	m.marketConnMu.RUnlock()

	if !ok {
		return
	}

	// Convert connection ID (7-150) to array index (0-143)
	if connID < 7 || connID > 150 {
		return
	}
	conn := m.orderbookConns[connID-7]

	conn.mu.Lock()
	delete(conn.markets, ticker)
	conn.mu.Unlock()

	// Find SID for this ticker and unsubscribe
	m.subsMu.RLock()
	var sid int64
	for s, sub := range m.subs {
		if sub.Ticker == ticker && sub.Channel == "orderbook_delta" {
			sid = s
			break
		}
	}
	m.subsMu.RUnlock()

	if sid != 0 {
		if err := m.unsubscribe(conn, sid); err != nil {
			m.logger.Warn("failed to unsubscribe orderbook",
				"ticker", ticker,
				"sid", sid,
				"error", err,
			)
		}
	}

	m.marketConnMu.Lock()
	delete(m.marketToConn, ticker)
	m.marketConnMu.Unlock()
}

// readLoop reads messages from a connection and routes them.
func (m *manager) readLoop(conn *connState) {
	defer m.wg.Done()
	defer close(conn.readLoopDone)

	for {
		select {
		case <-m.ctx.Done():
			return

		case err := <-conn.client.Errors():
			m.logger.Error("connection error, triggering reconnect",
				"conn", conn.id,
				"role", conn.role,
				"error", err,
			)
			m.wg.Add(1)
			go m.reconnect(conn)
			return

		case msg, ok := <-conn.client.Messages():
			if !ok {
				return
			}

			// Try to parse as command response
			if resp, ok := m.tryParseResponse(msg.Data); ok {
				conn.routeResponse(resp)
				continue
			}

			// Route lifecycle messages to Market Registry
			if conn.role == RoleLifecycle {
				select {
				case m.lifecycle <- msg.Data:
				case <-m.ctx.Done():
					return
				}
				continue
			}

			// Check sequence for orderbook messages
			var seqGap bool
			var gapSize int
			if conn.role == RoleOrderbook {
				if sid, seq, ok := m.extractSequence(msg.Data); ok {
					seqGap, gapSize = m.checkSequence(conn.id, sid, seq)
				}
			}

			// Data message - forward to router (non-blocking)
			rawMsg := RawMessage{
				Data:       msg.Data,
				ConnID:     conn.id,
				ReceivedAt: msg.ReceivedAt,
				SeqGap:     seqGap,
				GapSize:    gapSize,
			}

			select {
			case m.router <- rawMsg:
			case <-m.ctx.Done():
				return
			default:
				m.logger.Error("message buffer full, dropping message (data loss)",
					"conn", conn.id,
					"role", conn.role,
				)
			}
		}
	}
}

// tryParseResponse attempts to parse a message as a command response.
func (m *manager) tryParseResponse(data []byte) (Response, bool) {
	// Quick check for response markers
	if !bytes.Contains(data, []byte(`"id":`)) {
		return Response{}, false
	}

	var resp Response
	if err := json.Unmarshal(data, &resp); err != nil {
		return Response{}, false
	}

	// Valid response types
	switch resp.Type {
	case "subscribed", "unsubscribed", "error", "ok":
		return resp, true
	}

	return Response{}, false
}

// routeResponse sends a response to the waiting goroutine.
func (c *connState) routeResponse(resp Response) {
	c.pendingMu.Lock()
	ch, ok := c.pending[resp.ID]
	if ok {
		delete(c.pending, resp.ID)
	}
	c.pendingMu.Unlock()

	if ok {
		select {
		case ch <- resp:
		default:
		}
	}
}

// extractSequence extracts SID and sequence number from a data message.
func (m *manager) extractSequence(data []byte) (sid int64, seq int64, ok bool) {
	var msg DataMessage
	if err := json.Unmarshal(data, &msg); err != nil {
		return 0, 0, false
	}

	if msg.Seq == 0 {
		return 0, 0, false
	}

	return msg.SID, msg.Seq, true
}

// checkSequence checks for sequence gaps and returns gap info.
func (m *manager) checkSequence(connID int, sid int64, seq int64) (seqGap bool, gapSize int) {
	m.seqMu.Lock()
	defer m.seqMu.Unlock()

	key := seqKey{connID: connID, sid: sid}
	last, exists := m.lastSeq[key]
	if !exists {
		// First message for this subscription
		m.lastSeq[key] = seq
		return false, 0
	}

	if seq != last+1 {
		gap := int(seq - last - 1)
		if gap < 0 {
			// Sequence went backwards - likely reset or out-of-order
			m.logger.Warn("sequence reset or out-of-order",
				"conn_id", connID,
				"sid", sid,
				"expected", last+1,
				"got", seq,
			)
			m.lastSeq[key] = seq
			return true, 0
		}
		m.logger.Warn("sequence gap detected",
			"conn_id", connID,
			"sid", sid,
			"expected", last+1,
			"got", seq,
			"gap", gap,
		)
		m.lastSeq[key] = seq
		return true, gap
	}

	m.lastSeq[key] = seq
	return false, 0
}

// subscribe sends a subscribe command and waits for response.
func (m *manager) subscribe(conn *connState, channel, ticker string) error {
	id := atomic.AddInt64(&conn.cmdID, 1)
	respCh := make(chan Response, 1)

	conn.pendingMu.Lock()
	conn.pending[id] = respCh
	conn.pendingMu.Unlock()

	defer func() {
		conn.pendingMu.Lock()
		delete(conn.pending, id)
		conn.pendingMu.Unlock()
	}()

	// Build command
	params := map[string]interface{}{
		"channels": []string{channel},
	}
	if ticker != "" {
		params["market_ticker"] = ticker
	}

	cmd := Command{
		ID:     id,
		Cmd:    "subscribe",
		Params: params,
	}

	data, err := json.Marshal(cmd)
	if err != nil {
		return fmt.Errorf("marshal subscribe command: %w", err)
	}
	if err := conn.client.Send(data); err != nil {
		return err
	}

	// Wait for response
	select {
	case <-m.ctx.Done():
		return m.ctx.Err()
	case <-time.After(m.cfg.SubscribeTimeout):
		return ErrTimeout
	case resp := <-respCh:
		if resp.Type == "error" {
			var errMsg ErrorMsg
			if err := json.Unmarshal(resp.Msg, &errMsg); err != nil {
				return fmt.Errorf("error response (failed to parse: %v)", err)
			}
			return fmt.Errorf("%s: %s", errMsg.Code, errMsg.Message)
		}

		// Track subscription
		var subMsg SubscribedMsg
		if err := json.Unmarshal(resp.Msg, &subMsg); err != nil {
			return fmt.Errorf("parse subscribed response: %w", err)
		}

		m.subsMu.Lock()
		m.subs[subMsg.SID] = &Subscription{
			SID:     subMsg.SID,
			Channel: channel,
			ConnID:  conn.id,
			Ticker:  ticker,
		}
		m.subsMu.Unlock()

		m.logger.Debug("subscribed",
			"channel", channel,
			"ticker", ticker,
			"sid", subMsg.SID,
			"conn", conn.id,
		)

		return nil
	}
}

// unsubscribe sends an unsubscribe command and waits for response.
func (m *manager) unsubscribe(conn *connState, sid int64) error {
	id := atomic.AddInt64(&conn.cmdID, 1)
	respCh := make(chan Response, 1)

	conn.pendingMu.Lock()
	conn.pending[id] = respCh
	conn.pendingMu.Unlock()

	defer func() {
		conn.pendingMu.Lock()
		delete(conn.pending, id)
		conn.pendingMu.Unlock()
	}()

	cmd := Command{
		ID:  id,
		Cmd: "unsubscribe",
		Params: UnsubscribeParams{
			SIDs: []int64{sid},
		},
	}

	data, err := json.Marshal(cmd)
	if err != nil {
		return fmt.Errorf("marshal unsubscribe command: %w", err)
	}
	if err := conn.client.Send(data); err != nil {
		return err
	}

	select {
	case <-m.ctx.Done():
		return m.ctx.Err()
	case <-time.After(m.cfg.SubscribeTimeout):
		return ErrTimeout
	case resp := <-respCh:
		if resp.Type == "error" {
			var errMsg ErrorMsg
			if err := json.Unmarshal(resp.Msg, &errMsg); err != nil {
				return fmt.Errorf("error response (failed to parse: %v)", err)
			}
			return fmt.Errorf("%s: %s", errMsg.Code, errMsg.Message)
		}

		m.subsMu.Lock()
		delete(m.subs, sid)
		m.subsMu.Unlock()

		// Clean up sequence tracking
		m.seqMu.Lock()
		delete(m.lastSeq, seqKey{connID: conn.id, sid: sid})
		m.seqMu.Unlock()

		return nil
	}
}

// reconnect attempts to reconnect a connection with exponential backoff.
func (m *manager) reconnect(conn *connState) {
	defer m.wg.Done()

	wait := m.cfg.ReconnectBaseWait
	maxWait := m.cfg.ReconnectMaxWait

	for {
		select {
		case <-m.ctx.Done():
			return
		case <-time.After(wait):
		}

		m.logger.Info("attempting reconnection",
			"conn", conn.id,
			"role", conn.role,
		)

		// Close old connection and wait for old read loop to finish
		conn.client.Close()

		// Wait for old read loop to exit (with timeout to prevent deadlock)
		select {
		case <-conn.readLoopDone:
			// Old read loop finished
		case <-time.After(5 * time.Second):
			m.logger.Warn("timeout waiting for old read loop to finish", "conn", conn.id)
		case <-m.ctx.Done():
			return
		}

		// Create new client (safe now that old read loop is done)
		cfg := ClientConfig{
			URL:          m.cfg.WSURL,
			KeyID:        m.cfg.KeyID,
			PrivateKey:   m.cfg.PrivateKey,
			PingTimeout:  5 * time.Minute,
			WriteTimeout: 5 * time.Second,
			BufferSize:   100000,
		}

		conn.mu.Lock()
		conn.client = NewClient(cfg, m.logger.With("conn_id", conn.id, "role", conn.role))
		conn.readLoopDone = make(chan struct{})
		conn.pending = make(map[int64]chan Response)
		conn.mu.Unlock()

		if err := conn.client.Connect(m.ctx); err != nil {
			m.logger.Warn("reconnection failed",
				"conn", conn.id,
				"error", err,
			)

			// Exponential backoff
			wait *= 2
			if wait > maxWait {
				wait = maxWait
			}
			continue
		}

		m.logger.Info("reconnected", "conn", conn.id)

		// Re-subscribe based on role
		var subscribeErr error
		switch conn.role {
		case RoleTicker:
			if err := m.subscribe(conn, "ticker", ""); err != nil {
				m.logger.Error("failed to resubscribe ticker after reconnect",
					"conn", conn.id,
					"error", err,
				)
				subscribeErr = err
			}
		case RoleTrade:
			if err := m.subscribe(conn, "trade", ""); err != nil {
				m.logger.Error("failed to resubscribe trade after reconnect",
					"conn", conn.id,
					"error", err,
				)
				subscribeErr = err
			}
		case RoleLifecycle:
			if err := m.subscribe(conn, "market_lifecycle_v2", ""); err != nil {
				m.logger.Error("failed to resubscribe lifecycle after reconnect",
					"conn", conn.id,
					"error", err,
				)
				subscribeErr = err
			}
		case RoleOrderbook:
			// Re-subscribe to all markets on this connection
			conn.mu.Lock()
			markets := make([]string, 0, len(conn.markets))
			for ticker := range conn.markets {
				markets = append(markets, ticker)
			}
			conn.mu.Unlock()

			var failedCount int
			for _, ticker := range markets {
				if err := m.subscribe(conn, "orderbook_delta", ticker); err != nil {
					m.logger.Error("failed to resubscribe orderbook after reconnect",
						"conn", conn.id,
						"ticker", ticker,
						"error", err,
					)
					failedCount++
				}
			}
			if failedCount > 0 {
				m.logger.Error("orderbook resubscription partially failed",
					"conn", conn.id,
					"failed", failedCount,
					"total", len(markets),
				)
			}
		}

		// If subscription failed, retry the whole reconnect
		if subscribeErr != nil {
			wait = m.cfg.ReconnectBaseWait
			continue
		}

		// Restart read loop
		m.wg.Add(1)
		go m.readLoop(conn)

		return
	}
}
