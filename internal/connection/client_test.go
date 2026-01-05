package connection

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/gorilla/websocket"
)

// mockWSServer creates a test WebSocket server.
func mockWSServer(t *testing.T, handler func(*websocket.Conn)) *httptest.Server {
	upgrader := websocket.Upgrader{
		CheckOrigin: func(r *http.Request) bool { return true },
	}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		conn, err := upgrader.Upgrade(w, r, nil)
		if err != nil {
			t.Logf("upgrade error: %v", err)
			return
		}
		defer conn.Close()
		handler(conn)
	}))

	return server
}

func wsURL(server *httptest.Server) string {
	return "ws" + strings.TrimPrefix(server.URL, "http")
}

func TestClient_Connect(t *testing.T) {
	server := mockWSServer(t, func(conn *websocket.Conn) {
		// Just keep the connection open
		for {
			_, _, err := conn.ReadMessage()
			if err != nil {
				return
			}
		}
	})
	defer server.Close()

	cfg := ClientConfig{
		URL:          wsURL(server),
		PingTimeout:  30 * time.Second,
		WriteTimeout: 5 * time.Second,
		BufferSize:   100,
	}

	client := NewClient(cfg, nil)
	ctx := context.Background()

	err := client.Connect(ctx)
	if err != nil {
		t.Fatalf("Connect failed: %v", err)
	}

	if !client.IsConnected() {
		t.Error("expected IsConnected to return true")
	}

	err = client.Close()
	if err != nil {
		t.Errorf("Close failed: %v", err)
	}

	if client.IsConnected() {
		t.Error("expected IsConnected to return false after Close")
	}
}

func TestClient_Send(t *testing.T) {
	var received []byte
	var mu sync.Mutex

	server := mockWSServer(t, func(conn *websocket.Conn) {
		for {
			_, msg, err := conn.ReadMessage()
			if err != nil {
				return
			}
			mu.Lock()
			received = msg
			mu.Unlock()
		}
	})
	defer server.Close()

	cfg := ClientConfig{
		URL:          wsURL(server),
		PingTimeout:  30 * time.Second,
		WriteTimeout: 5 * time.Second,
		BufferSize:   100,
	}

	client := NewClient(cfg, nil)
	ctx := context.Background()

	if err := client.Connect(ctx); err != nil {
		t.Fatalf("Connect failed: %v", err)
	}
	defer client.Close()

	testMsg := []byte(`{"test": "message"}`)
	if err := client.Send(testMsg); err != nil {
		t.Errorf("Send failed: %v", err)
	}

	// Wait for message to be received
	time.Sleep(50 * time.Millisecond)

	mu.Lock()
	defer mu.Unlock()
	if string(received) != string(testMsg) {
		t.Errorf("received %q, want %q", received, testMsg)
	}
}

func TestClient_Messages(t *testing.T) {
	testMessages := []string{
		`{"type": "test", "data": 1}`,
		`{"type": "test", "data": 2}`,
		`{"type": "test", "data": 3}`,
	}

	server := mockWSServer(t, func(conn *websocket.Conn) {
		for _, msg := range testMessages {
			if err := conn.WriteMessage(websocket.TextMessage, []byte(msg)); err != nil {
				return
			}
			time.Sleep(10 * time.Millisecond)
		}
		// Keep connection open
		time.Sleep(time.Second)
	})
	defer server.Close()

	cfg := ClientConfig{
		URL:          wsURL(server),
		PingTimeout:  30 * time.Second,
		WriteTimeout: 5 * time.Second,
		BufferSize:   100,
	}

	client := NewClient(cfg, nil)
	ctx := context.Background()

	if err := client.Connect(ctx); err != nil {
		t.Fatalf("Connect failed: %v", err)
	}
	defer client.Close()

	// Collect received messages
	var received []string
	timeout := time.After(500 * time.Millisecond)

	for i := 0; i < len(testMessages); i++ {
		select {
		case msg := <-client.Messages():
			received = append(received, string(msg.Data))
			if msg.ReceivedAt.IsZero() {
				t.Error("ReceivedAt should not be zero")
			}
		case <-timeout:
			t.Fatalf("timeout waiting for messages, received %d of %d", len(received), len(testMessages))
		}
	}

	for i, want := range testMessages {
		if received[i] != want {
			t.Errorf("message %d: got %q, want %q", i, received[i], want)
		}
	}
}

func TestClient_SendNotConnected(t *testing.T) {
	cfg := ClientConfig{
		URL:          "ws://localhost:12345",
		PingTimeout:  30 * time.Second,
		WriteTimeout: 5 * time.Second,
		BufferSize:   100,
	}

	client := NewClient(cfg, nil)

	err := client.Send([]byte("test"))
	if err != ErrNotConnected {
		t.Errorf("expected ErrNotConnected, got %v", err)
	}
}

func TestClient_DoubleClose(t *testing.T) {
	server := mockWSServer(t, func(conn *websocket.Conn) {
		time.Sleep(time.Second)
	})
	defer server.Close()

	cfg := ClientConfig{
		URL:          wsURL(server),
		PingTimeout:  30 * time.Second,
		WriteTimeout: 5 * time.Second,
		BufferSize:   100,
	}

	client := NewClient(cfg, nil)
	ctx := context.Background()

	if err := client.Connect(ctx); err != nil {
		t.Fatalf("Connect failed: %v", err)
	}

	// First close should succeed
	if err := client.Close(); err != nil {
		t.Errorf("first Close failed: %v", err)
	}

	// Second close should be no-op
	if err := client.Close(); err != nil {
		t.Errorf("second Close failed: %v", err)
	}
}

func TestClient_PingHandler(t *testing.T) {
	server := mockWSServer(t, func(conn *websocket.Conn) {
		// Send ping
		if err := conn.WriteControl(websocket.PingMessage, []byte("heartbeat"), time.Now().Add(time.Second)); err != nil {
			t.Logf("ping error: %v", err)
			return
		}
		// Wait for pong (handled automatically by gorilla/websocket on the client side,
		// but we set our own handler which updates lastPingAt)
		time.Sleep(500 * time.Millisecond)
	})
	defer server.Close()

	cfg := ClientConfig{
		URL:          wsURL(server),
		PingTimeout:  30 * time.Second,
		WriteTimeout: 5 * time.Second,
		BufferSize:   100,
	}

	client := NewClient(cfg, nil)
	ctx := context.Background()

	if err := client.Connect(ctx); err != nil {
		t.Fatalf("Connect failed: %v", err)
	}
	defer client.Close()

	// Give time for ping to be processed
	time.Sleep(200 * time.Millisecond)

	// Client should still be connected
	if !client.IsConnected() {
		t.Error("expected client to be connected after ping")
	}
}

func TestTypes_Command(t *testing.T) {
	cmd := Command{
		ID:  1,
		Cmd: "subscribe",
		Params: SubscribeParams{
			Channels:     []string{"orderbook_delta"},
			MarketTicker: "TEST-MARKET",
		},
	}

	data, err := json.Marshal(cmd)
	if err != nil {
		t.Fatalf("marshal failed: %v", err)
	}

	var parsed Command
	if err := json.Unmarshal(data, &parsed); err != nil {
		t.Fatalf("unmarshal failed: %v", err)
	}

	if parsed.ID != 1 {
		t.Errorf("ID = %d, want 1", parsed.ID)
	}
	if parsed.Cmd != "subscribe" {
		t.Errorf("Cmd = %s, want subscribe", parsed.Cmd)
	}
}

func TestTypes_Response(t *testing.T) {
	data := `{"id":1,"type":"subscribed","msg":{"sid":42,"channel":"orderbook_delta"}}`

	var resp Response
	if err := json.Unmarshal([]byte(data), &resp); err != nil {
		t.Fatalf("unmarshal failed: %v", err)
	}

	if resp.ID != 1 {
		t.Errorf("ID = %d, want 1", resp.ID)
	}
	if resp.Type != "subscribed" {
		t.Errorf("Type = %s, want subscribed", resp.Type)
	}

	var subMsg SubscribedMsg
	if err := json.Unmarshal(resp.Msg, &subMsg); err != nil {
		t.Fatalf("unmarshal msg failed: %v", err)
	}

	if subMsg.SID != 42 {
		t.Errorf("SID = %d, want 42", subMsg.SID)
	}
	if subMsg.Channel != "orderbook_delta" {
		t.Errorf("Channel = %s, want orderbook_delta", subMsg.Channel)
	}
}

func TestTypes_DataMessage(t *testing.T) {
	data := `{"type":"orderbook_delta","sid":1,"seq":100,"msg":{"ticker":"TEST"}}`

	var msg DataMessage
	if err := json.Unmarshal([]byte(data), &msg); err != nil {
		t.Fatalf("unmarshal failed: %v", err)
	}

	if msg.Type != "orderbook_delta" {
		t.Errorf("Type = %s, want orderbook_delta", msg.Type)
	}
	if msg.SID != 1 {
		t.Errorf("SID = %d, want 1", msg.SID)
	}
	if msg.Seq != 100 {
		t.Errorf("Seq = %d, want 100", msg.Seq)
	}
}

func TestTypes_LifecycleMsg(t *testing.T) {
	data := `{"market_ticker":"TEST","event_type":"status_change","old_status":"active","new_status":"closed","result":"","ts":1705328200}`

	var msg LifecycleMsg
	if err := json.Unmarshal([]byte(data), &msg); err != nil {
		t.Fatalf("unmarshal failed: %v", err)
	}

	if msg.MarketTicker != "TEST" {
		t.Errorf("MarketTicker = %s, want TEST", msg.MarketTicker)
	}
	if msg.EventType != "status_change" {
		t.Errorf("EventType = %s, want status_change", msg.EventType)
	}
	if msg.OldStatus != "active" {
		t.Errorf("OldStatus = %s, want active", msg.OldStatus)
	}
	if msg.NewStatus != "closed" {
		t.Errorf("NewStatus = %s, want closed", msg.NewStatus)
	}
	if msg.Timestamp != 1705328200 {
		t.Errorf("Timestamp = %d, want 1705328200", msg.Timestamp)
	}
}

func TestDefaultConfigs(t *testing.T) {
	clientCfg := DefaultClientConfig()
	if clientCfg.PingTimeout != 30*time.Second {
		t.Errorf("PingTimeout = %v, want 30s", clientCfg.PingTimeout)
	}
	if clientCfg.BufferSize != 1000 {
		t.Errorf("BufferSize = %d, want 1000", clientCfg.BufferSize)
	}

	mgrCfg := DefaultManagerConfig()
	if mgrCfg.SubscribeTimeout != 10*time.Second {
		t.Errorf("SubscribeTimeout = %v, want 10s", mgrCfg.SubscribeTimeout)
	}
	if mgrCfg.WorkerCount != 10 {
		t.Errorf("WorkerCount = %d, want 10", mgrCfg.WorkerCount)
	}
}
