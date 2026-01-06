package connection

import (
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/gorilla/websocket"
	"github.com/rickgao/kalshi-data/internal/market"
	"github.com/rickgao/kalshi-data/internal/model"
)

// mockRegistry implements market.Registry for testing.
type mockRegistry struct {
	mu              sync.Mutex
	activeMarkets   []model.Market
	changes         chan market.MarketChange
	lifecycleSource <-chan []byte
}

func newMockRegistry() *mockRegistry {
	return &mockRegistry{
		activeMarkets: []model.Market{},
		changes:       make(chan market.MarketChange, 100),
	}
}

func (r *mockRegistry) Start(ctx context.Context) error              { return nil }
func (r *mockRegistry) Stop(ctx context.Context) error               { return nil }
func (r *mockRegistry) GetActiveMarkets() []model.Market             { return r.activeMarkets }
func (r *mockRegistry) GetMarket(ticker string) (model.Market, bool) { return model.Market{}, false }
func (r *mockRegistry) SubscribeChanges() <-chan market.MarketChange { return r.changes }
func (r *mockRegistry) SetLifecycleSource(ch <-chan []byte)          { r.lifecycleSource = ch }

func (r *mockRegistry) AddMarket(m model.Market) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.activeMarkets = append(r.activeMarkets, m)
}

func (r *mockRegistry) SendChange(change market.MarketChange) {
	r.changes <- change
}

// mockWSServerMulti creates a test WebSocket server that handles multiple connections.
func mockWSServerMulti(t *testing.T, handler func(int, *websocket.Conn)) *httptest.Server {
	upgrader := websocket.Upgrader{
		CheckOrigin: func(r *http.Request) bool { return true },
	}

	var mu sync.Mutex
	connCount := 0

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		conn, err := upgrader.Upgrade(w, r, nil)
		if err != nil {
			t.Logf("upgrade error: %v", err)
			return
		}
		defer conn.Close()

		mu.Lock()
		connCount++
		id := connCount
		mu.Unlock()

		handler(id, conn)
	}))

	return server
}

func TestManager_Start_Stop(t *testing.T) {
	server := mockWSServerMulti(t, func(id int, conn *websocket.Conn) {
		// Echo server - respond to subscribes
		for {
			_, msg, err := conn.ReadMessage()
			if err != nil {
				return
			}

			var cmd Command
			if err := json.Unmarshal(msg, &cmd); err != nil {
				continue
			}

			if cmd.Cmd == "subscribe" {
				resp := Response{
					ID:   cmd.ID,
					Type: "subscribed",
					Msg:  json.RawMessage(`{"sid":1,"channel":"ticker"}`),
				}
				data, _ := json.Marshal(resp)
				conn.WriteMessage(websocket.TextMessage, data)
			}
		}
	})
	defer server.Close()

	registry := newMockRegistry()

	cfg := ManagerConfig{
		WSURL:             wsURL(server),
		SubscribeTimeout:  5 * time.Second,
		ReconnectBaseWait: 100 * time.Millisecond,
		ReconnectMaxWait:  1 * time.Second,
		MessageBufferSize: 1000,
		WorkerCount:       2,
	}

	mgr := NewManager(cfg, registry, nil)

	ctx := context.Background()
	if err := mgr.Start(ctx); err != nil {
		t.Fatalf("Start failed: %v", err)
	}

	// Give time for connections
	time.Sleep(100 * time.Millisecond)

	stats := mgr.Stats()
	if stats.ConnectedCount == 0 {
		t.Error("expected some connections")
	}

	stopCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := mgr.Stop(stopCtx); err != nil {
		t.Errorf("Stop failed: %v", err)
	}
}

func TestManager_SubscribeOrderbook(t *testing.T) {
	var subscribeCount int
	var mu sync.Mutex

	server := mockWSServerMulti(t, func(id int, conn *websocket.Conn) {
		for {
			_, msg, err := conn.ReadMessage()
			if err != nil {
				return
			}

			var cmd Command
			if err := json.Unmarshal(msg, &cmd); err != nil {
				continue
			}

			if cmd.Cmd == "subscribe" {
				mu.Lock()
				subscribeCount++
				sid := subscribeCount
				mu.Unlock()

				resp := Response{
					ID:   cmd.ID,
					Type: "subscribed",
				}

				// Check what channel is being subscribed
				params := cmd.Params.(map[string]interface{})
				channels := params["channels"].([]interface{})
				channel := channels[0].(string)

				subMsg := SubscribedMsg{
					SID:     int64(sid),
					Channel: channel,
				}
				msgData, _ := json.Marshal(subMsg)
				resp.Msg = msgData

				data, _ := json.Marshal(resp)
				conn.WriteMessage(websocket.TextMessage, data)
			}
		}
	})
	defer server.Close()

	// Create registry with some active markets
	registry := newMockRegistry()
	registry.AddMarket(model.Market{Ticker: "TEST-1", MarketStatus: "open"})
	registry.AddMarket(model.Market{Ticker: "TEST-2", MarketStatus: "open"})

	cfg := ManagerConfig{
		WSURL:             wsURL(server),
		SubscribeTimeout:  5 * time.Second,
		ReconnectBaseWait: 100 * time.Millisecond,
		ReconnectMaxWait:  1 * time.Second,
		MessageBufferSize: 1000,
		WorkerCount:       2,
	}

	mgr := NewManager(cfg, registry, nil)

	ctx := context.Background()
	if err := mgr.Start(ctx); err != nil {
		t.Fatalf("Start failed: %v", err)
	}
	defer func() {
		stopCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		mgr.Stop(stopCtx)
	}()

	// Give time for subscriptions
	time.Sleep(500 * time.Millisecond)

	stats := mgr.Stats()
	// Should have subscribed to global channels (ticker, trade, lifecycle) + 2 orderbooks
	// Global: 2 ticker + 2 trade + 2 lifecycle = 6
	// Orderbooks: 2
	// Total: 8
	if stats.TotalSubscriptions < 2 {
		t.Errorf("TotalSubscriptions = %d, want >= 2", stats.TotalSubscriptions)
	}

	if stats.MarketsSubscribed != 2 {
		t.Errorf("MarketsSubscribed = %d, want 2", stats.MarketsSubscribed)
	}
}

func TestManager_HandleMarketChange_Created(t *testing.T) {
	var subscriptions []string
	var mu sync.Mutex

	server := mockWSServerMulti(t, func(id int, conn *websocket.Conn) {
		for {
			_, msg, err := conn.ReadMessage()
			if err != nil {
				return
			}

			var cmd Command
			if err := json.Unmarshal(msg, &cmd); err != nil {
				continue
			}

			if cmd.Cmd == "subscribe" {
				params := cmd.Params.(map[string]interface{})
				if ticker, ok := params["market_ticker"].(string); ok {
					mu.Lock()
					subscriptions = append(subscriptions, ticker)
					mu.Unlock()
				}

				resp := Response{
					ID:   cmd.ID,
					Type: "subscribed",
					Msg:  json.RawMessage(`{"sid":1,"channel":"orderbook_delta"}`),
				}
				data, _ := json.Marshal(resp)
				conn.WriteMessage(websocket.TextMessage, data)
			}
		}
	})
	defer server.Close()

	registry := newMockRegistry()

	cfg := ManagerConfig{
		WSURL:             wsURL(server),
		SubscribeTimeout:  5 * time.Second,
		ReconnectBaseWait: 100 * time.Millisecond,
		ReconnectMaxWait:  1 * time.Second,
		MessageBufferSize: 1000,
		WorkerCount:       2,
	}

	mgr := NewManager(cfg, registry, nil)

	ctx := context.Background()
	if err := mgr.Start(ctx); err != nil {
		t.Fatalf("Start failed: %v", err)
	}
	defer func() {
		stopCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		mgr.Stop(stopCtx)
	}()

	// Give time for initial setup
	time.Sleep(200 * time.Millisecond)

	// Send a market created event
	newMarket := model.Market{Ticker: "NEW-MARKET", MarketStatus: "open"}
	registry.SendChange(market.MarketChange{
		Ticker:    "NEW-MARKET",
		EventType: "created",
		NewStatus: "open",
		Market:    &newMarket,
	})

	// Wait for subscription
	time.Sleep(300 * time.Millisecond)

	mu.Lock()
	defer mu.Unlock()

	found := false
	for _, ticker := range subscriptions {
		if ticker == "NEW-MARKET" {
			found = true
			break
		}
	}

	if !found {
		t.Errorf("expected subscription to NEW-MARKET, got subscriptions: %v", subscriptions)
	}
}

func TestManager_MessageRouting(t *testing.T) {
	server := mockWSServerMulti(t, func(id int, conn *websocket.Conn) {
		// Respond to subscribes
		for {
			_, msg, err := conn.ReadMessage()
			if err != nil {
				return
			}

			var cmd Command
			if err := json.Unmarshal(msg, &cmd); err != nil {
				continue
			}

			if cmd.Cmd == "subscribe" {
				resp := Response{
					ID:   cmd.ID,
					Type: "subscribed",
					Msg:  json.RawMessage(`{"sid":1,"channel":"ticker"}`),
				}
				data, _ := json.Marshal(resp)
				conn.WriteMessage(websocket.TextMessage, data)

				// Send a data message
				dataMsg := `{"type":"ticker","sid":1,"msg":{"ticker":"TEST","yes_bid":50}}`
				conn.WriteMessage(websocket.TextMessage, []byte(dataMsg))
			}
		}
	})
	defer server.Close()

	registry := newMockRegistry()

	cfg := ManagerConfig{
		WSURL:             wsURL(server),
		SubscribeTimeout:  5 * time.Second,
		ReconnectBaseWait: 100 * time.Millisecond,
		ReconnectMaxWait:  1 * time.Second,
		MessageBufferSize: 1000,
		WorkerCount:       2,
	}

	mgr := NewManager(cfg, registry, nil)

	ctx := context.Background()
	if err := mgr.Start(ctx); err != nil {
		t.Fatalf("Start failed: %v", err)
	}
	defer func() {
		stopCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		mgr.Stop(stopCtx)
	}()

	// Read from messages channel
	select {
	case msg := <-mgr.Messages():
		if !strings.Contains(string(msg.Data), "ticker") {
			t.Errorf("expected ticker message, got: %s", msg.Data)
		}
		if msg.ConnID == 0 {
			t.Error("expected non-zero ConnID")
		}
		if msg.ReceivedAt.IsZero() {
			t.Error("expected non-zero ReceivedAt")
		}
	case <-time.After(2 * time.Second):
		t.Error("timeout waiting for message")
	}
}

func TestManager_SequenceGapDetection(t *testing.T) {
	mgr := &manager{
		lastSeq: make(map[seqKey]int64),
		logger:  slog.Default(),
	}

	// First message - no gap (connID=7, sid=1, seq=1)
	gap, size := mgr.checkSequence(7, 1, 1)
	if gap {
		t.Error("expected no gap for first message")
	}
	if size != 0 {
		t.Errorf("expected gap size 0, got %d", size)
	}

	// Sequential message - no gap
	gap, size = mgr.checkSequence(7, 1, 2)
	if gap {
		t.Error("expected no gap for sequential message")
	}

	// Gap - skipped sequence 3,4
	gap, size = mgr.checkSequence(7, 1, 5)
	if !gap {
		t.Error("expected gap detected")
	}
	if size != 2 {
		t.Errorf("expected gap size 2, got %d", size)
	}

	// Different SID on same connection - no gap
	gap, size = mgr.checkSequence(7, 2, 10)
	if gap {
		t.Error("expected no gap for new SID")
	}

	// Same SID on different connection - no gap (SIDs are per-connection)
	gap, size = mgr.checkSequence(8, 1, 1)
	if gap {
		t.Error("expected no gap for same SID on different connection")
	}
}

func TestManager_TryParseResponse(t *testing.T) {
	mgr := &manager{}

	tests := []struct {
		name     string
		data     string
		wantOK   bool
		wantType string
	}{
		{
			name:     "subscribed response",
			data:     `{"id":1,"type":"subscribed","msg":{"sid":1}}`,
			wantOK:   true,
			wantType: "subscribed",
		},
		{
			name:     "unsubscribed response",
			data:     `{"id":2,"type":"unsubscribed","msg":{"sids":[1]}}`,
			wantOK:   true,
			wantType: "unsubscribed",
		},
		{
			name:     "error response",
			data:     `{"id":3,"type":"error","msg":{"code":"ERR"}}`,
			wantOK:   true,
			wantType: "error",
		},
		{
			name:   "data message (not a response)",
			data:   `{"type":"ticker","sid":1,"msg":{}}`,
			wantOK: false,
		},
		{
			name:   "invalid json",
			data:   `not json`,
			wantOK: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			resp, ok := mgr.tryParseResponse([]byte(tt.data))
			if ok != tt.wantOK {
				t.Errorf("tryParseResponse() ok = %v, want %v", ok, tt.wantOK)
			}
			if ok && resp.Type != tt.wantType {
				t.Errorf("tryParseResponse() type = %s, want %s", resp.Type, tt.wantType)
			}
		})
	}
}

func TestManager_ExtractSequence(t *testing.T) {
	mgr := &manager{}

	tests := []struct {
		name    string
		data    string
		wantSID int64
		wantSeq int64
		wantOK  bool
	}{
		{
			name:    "valid message with sequence",
			data:    `{"type":"orderbook_delta","sid":42,"seq":100,"msg":{}}`,
			wantSID: 42,
			wantSeq: 100,
			wantOK:  true,
		},
		{
			name:   "message without sequence",
			data:   `{"type":"ticker","sid":1,"msg":{}}`,
			wantOK: false,
		},
		{
			name:   "invalid json",
			data:   `not json`,
			wantOK: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			sid, seq, ok := mgr.extractSequence([]byte(tt.data))
			if ok != tt.wantOK {
				t.Errorf("extractSequence() ok = %v, want %v", ok, tt.wantOK)
			}
			if ok {
				if sid != tt.wantSID {
					t.Errorf("extractSequence() sid = %d, want %d", sid, tt.wantSID)
				}
				if seq != tt.wantSeq {
					t.Errorf("extractSequence() seq = %d, want %d", seq, tt.wantSeq)
				}
			}
		})
	}
}

func TestConnState_RouteResponse(t *testing.T) {
	conn := &connState{
		pending: make(map[int64]chan Response),
	}

	// Set up pending request
	respCh := make(chan Response, 1)
	conn.pending[1] = respCh

	// Route response
	resp := Response{ID: 1, Type: "subscribed"}
	conn.routeResponse(resp)

	// Check response was received
	select {
	case received := <-respCh:
		if received.ID != 1 {
			t.Errorf("ID = %d, want 1", received.ID)
		}
	default:
		t.Error("expected response in channel")
	}

	// Pending should be cleaned up
	if _, ok := conn.pending[1]; ok {
		t.Error("expected pending entry to be deleted")
	}
}

func TestIsActiveStatus(t *testing.T) {
	tests := []struct {
		status string
		want   bool
	}{
		{"open", true},
		{"active", true},
		{"closed", false},
		{"settled", false},
		{"unopened", false},
		{"", false},
	}

	for _, tt := range tests {
		t.Run(tt.status, func(t *testing.T) {
			if got := isActiveStatus(tt.status); got != tt.want {
				t.Errorf("isActiveStatus(%q) = %v, want %v", tt.status, got, tt.want)
			}
		})
	}
}
