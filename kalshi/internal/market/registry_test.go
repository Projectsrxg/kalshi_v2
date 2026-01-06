package market

import (
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"

	"github.com/rickgao/kalshi-data/internal/api"
	"github.com/rickgao/kalshi-data/internal/model"
)

func TestState_UpsertAndGet(t *testing.T) {
	s := newState()

	m := model.Market{
		Ticker:       "TEST-MARKET",
		EventTicker:  "TEST-EVENT",
		Title:        "Test Market",
		MarketStatus: "active",
	}

	s.upsertMarket(m)

	got, ok := s.getMarket("TEST-MARKET")
	if !ok {
		t.Fatal("market not found")
	}
	if got.Ticker != "TEST-MARKET" {
		t.Errorf("Ticker = %q, want %q", got.Ticker, "TEST-MARKET")
	}
	if got.MarketStatus != "active" {
		t.Errorf("MarketStatus = %q, want %q", got.MarketStatus, "active")
	}
}

func TestState_GetMarket_NotFound(t *testing.T) {
	s := newState()

	_, ok := s.getMarket("NONEXISTENT")
	if ok {
		t.Error("expected market not found")
	}
}

func TestState_ActiveMarkets(t *testing.T) {
	s := newState()

	markets := []model.Market{
		{Ticker: "ACTIVE-1", MarketStatus: "active"},
		{Ticker: "ACTIVE-2", MarketStatus: "open"},
		{Ticker: "CLOSED-1", MarketStatus: "closed"},
		{Ticker: "SETTLED-1", MarketStatus: "finalized"},
	}

	for _, m := range markets {
		s.upsertMarket(m)
	}

	active := s.getActiveMarkets()
	if len(active) != 2 {
		t.Errorf("len(active) = %d, want 2", len(active))
	}

	// Verify active markets are the right ones.
	activeMap := make(map[string]bool)
	for _, m := range active {
		activeMap[m.Ticker] = true
	}

	if !activeMap["ACTIVE-1"] {
		t.Error("ACTIVE-1 should be active")
	}
	if !activeMap["ACTIVE-2"] {
		t.Error("ACTIVE-2 should be active")
	}
}

func TestState_ActiveMarkets_Empty(t *testing.T) {
	s := newState()

	active := s.getActiveMarkets()
	if len(active) != 0 {
		t.Errorf("len(active) = %d, want 0", len(active))
	}
}

func TestState_UpdateStatus(t *testing.T) {
	s := newState()

	m := model.Market{
		Ticker:       "TEST-MARKET",
		MarketStatus: "active",
	}
	s.upsertMarket(m)

	// Verify initially active.
	active := s.getActiveMarkets()
	if len(active) != 1 {
		t.Errorf("len(active) = %d, want 1", len(active))
	}

	// Update to closed.
	oldStatus, found := s.updateStatus("TEST-MARKET", "closed")
	if !found {
		t.Fatal("market not found")
	}
	if oldStatus != "active" {
		t.Errorf("oldStatus = %q, want %q", oldStatus, "active")
	}

	// Verify no longer active.
	active = s.getActiveMarkets()
	if len(active) != 0 {
		t.Errorf("len(active) = %d, want 0", len(active))
	}

	// Verify status updated.
	got, _ := s.getMarket("TEST-MARKET")
	if got.MarketStatus != "closed" {
		t.Errorf("MarketStatus = %q, want %q", got.MarketStatus, "closed")
	}
}

func TestState_UpdateStatus_ToActive(t *testing.T) {
	s := newState()

	m := model.Market{
		Ticker:       "TEST-MARKET",
		MarketStatus: "closed",
	}
	s.upsertMarket(m)

	// Verify initially not active.
	active := s.getActiveMarkets()
	if len(active) != 0 {
		t.Errorf("len(active) = %d, want 0", len(active))
	}

	// Update to active.
	oldStatus, found := s.updateStatus("TEST-MARKET", "active")
	if !found {
		t.Fatal("market not found")
	}
	if oldStatus != "closed" {
		t.Errorf("oldStatus = %q, want %q", oldStatus, "closed")
	}

	// Verify now active.
	active = s.getActiveMarkets()
	if len(active) != 1 {
		t.Errorf("len(active) = %d, want 1", len(active))
	}
}

func TestState_UpdateStatus_NotFound(t *testing.T) {
	s := newState()

	oldStatus, found := s.updateStatus("NONEXISTENT", "closed")
	if found {
		t.Error("expected market not found")
	}
	if oldStatus != "" {
		t.Errorf("oldStatus = %q, want empty", oldStatus)
	}
}

func TestState_NotifyChange(t *testing.T) {
	s := newState()

	change := MarketChange{
		Ticker:    "TEST-MARKET",
		EventType: "created",
		NewStatus: "active",
	}

	s.notifyChange(change)

	select {
	case got := <-s.changes:
		if got.Ticker != "TEST-MARKET" {
			t.Errorf("Ticker = %q, want %q", got.Ticker, "TEST-MARKET")
		}
	default:
		t.Error("expected change in channel")
	}
}

func TestState_NotifyChange_ChannelFull(t *testing.T) {
	s := newState()

	// Fill the channel.
	for i := 0; i < ChangeBufferSize; i++ {
		s.changes <- MarketChange{Ticker: "FILL"}
	}

	// This should drop the oldest and add new.
	change := MarketChange{
		Ticker:    "NEW-CHANGE",
		EventType: "created",
		NewStatus: "active",
	}
	s.notifyChange(change)

	// Drain and verify new change is there.
	found := false
	for i := 0; i < ChangeBufferSize; i++ {
		select {
		case c := <-s.changes:
			if c.Ticker == "NEW-CHANGE" {
				found = true
			}
		default:
			break
		}
	}
	if !found {
		t.Error("expected new change to be in channel")
	}
}

func TestState_NotifyChange_AllFields(t *testing.T) {
	s := newState()

	market := &model.Market{
		Ticker:       "TEST-MARKET",
		Title:        "Test Market",
		MarketStatus: "active",
	}

	change := MarketChange{
		Ticker:    "TEST-MARKET",
		EventType: "status_change",
		OldStatus: "closed",
		NewStatus: "active",
		Market:    market,
	}

	s.notifyChange(change)

	select {
	case got := <-s.changes:
		if got.Ticker != "TEST-MARKET" {
			t.Errorf("Ticker = %q, want %q", got.Ticker, "TEST-MARKET")
		}
		if got.EventType != "status_change" {
			t.Errorf("EventType = %q, want %q", got.EventType, "status_change")
		}
		if got.OldStatus != "closed" {
			t.Errorf("OldStatus = %q, want %q", got.OldStatus, "closed")
		}
		if got.NewStatus != "active" {
			t.Errorf("NewStatus = %q, want %q", got.NewStatus, "active")
		}
		if got.Market == nil {
			t.Error("Market should not be nil")
		}
	default:
		t.Error("expected change in channel")
	}
}

func TestIsActive(t *testing.T) {
	tests := []struct {
		status string
		want   bool
	}{
		{"active", true},
		{"open", true},
		{"closed", false},
		{"finalized", false},
		{"settled", false},
		{"initialized", false},
		{"inactive", false},
		{"determined", false},
		{"disputed", false},
		{"amended", false},
		{"", false},
		{"ACTIVE", false}, // case sensitive
		{"Active", false},
	}

	for _, tt := range tests {
		t.Run(tt.status, func(t *testing.T) {
			got := isActive(tt.status)
			if got != tt.want {
				t.Errorf("isActive(%q) = %v, want %v", tt.status, got, tt.want)
			}
		})
	}
}

func TestNewState(t *testing.T) {
	s := newState()

	if s == nil {
		t.Fatal("newState() returned nil")
	}
	if s.markets == nil {
		t.Error("markets map is nil")
	}
	if s.activeSet == nil {
		t.Error("activeSet map is nil")
	}
	if s.changes == nil {
		t.Error("changes channel is nil")
	}
	if cap(s.changes) != ChangeBufferSize {
		t.Errorf("changes capacity = %d, want %d", cap(s.changes), ChangeBufferSize)
	}
}

func TestState_UpsertMarket_UpdateExisting(t *testing.T) {
	s := newState()

	m1 := model.Market{
		Ticker:       "TEST-MARKET",
		Title:        "Original Title",
		MarketStatus: "active",
	}
	s.upsertMarket(m1)

	m2 := model.Market{
		Ticker:       "TEST-MARKET",
		Title:        "Updated Title",
		MarketStatus: "closed",
	}
	s.upsertMarket(m2)

	got, ok := s.getMarket("TEST-MARKET")
	if !ok {
		t.Fatal("market not found")
	}
	if got.Title != "Updated Title" {
		t.Errorf("Title = %q, want %q", got.Title, "Updated Title")
	}
	if got.MarketStatus != "closed" {
		t.Errorf("MarketStatus = %q, want %q", got.MarketStatus, "closed")
	}

	// Should no longer be active.
	active := s.getActiveMarkets()
	if len(active) != 0 {
		t.Errorf("len(active) = %d, want 0", len(active))
	}
}

func TestState_UpsertMarket_IsolatedCopy(t *testing.T) {
	s := newState()

	m := model.Market{
		Ticker:       "TEST-MARKET",
		Title:        "Original",
		MarketStatus: "active",
	}
	s.upsertMarket(m)

	// Modify original - should not affect stored.
	m.Title = "Modified"

	got, _ := s.getMarket("TEST-MARKET")
	if got.Title != "Original" {
		t.Errorf("Title = %q, want %q (should be isolated)", got.Title, "Original")
	}
}

func TestState_ConcurrentAccess(t *testing.T) {
	s := newState()

	// Pre-populate with some markets.
	for i := 0; i < 10; i++ {
		s.upsertMarket(model.Market{
			Ticker:       "MARKET-" + string(rune('A'+i)),
			MarketStatus: "active",
		})
	}

	var wg sync.WaitGroup
	errors := make(chan error, 100)

	// Concurrent reads.
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := 0; j < 100; j++ {
				s.getActiveMarkets()
				s.getMarket("MARKET-A")
			}
		}()
	}

	// Concurrent writes.
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			for j := 0; j < 100; j++ {
				s.upsertMarket(model.Market{
					Ticker:       "NEW-MARKET-" + string(rune('A'+id)),
					MarketStatus: "active",
				})
				s.updateStatus("MARKET-A", "active")
			}
		}(i)
	}

	wg.Wait()
	close(errors)

	for err := range errors {
		t.Error(err)
	}
}

func TestMarketChange_EventTypes(t *testing.T) {
	tests := []struct {
		eventType string
		valid     bool
	}{
		{"created", true},
		{"status_change", true},
		{"settled", true},
		{"unknown", true}, // No validation in struct
	}

	for _, tt := range tests {
		t.Run(tt.eventType, func(t *testing.T) {
			change := MarketChange{
				Ticker:    "TEST",
				EventType: tt.eventType,
			}
			if change.EventType != tt.eventType {
				t.Errorf("EventType = %q, want %q", change.EventType, tt.eventType)
			}
		})
	}
}

func TestChangeBufferSize(t *testing.T) {
	if ChangeBufferSize != 1000 {
		t.Errorf("ChangeBufferSize = %d, want 1000", ChangeBufferSize)
	}
}

// Tests for impl.go

func TestDefaultConfig(t *testing.T) {
	cfg := DefaultConfig()

	if cfg.ReconcileInterval != 5*time.Minute {
		t.Errorf("ReconcileInterval = %v, want %v", cfg.ReconcileInterval, 5*time.Minute)
	}
	if cfg.PageSize != 1000 {
		t.Errorf("PageSize = %d, want 1000", cfg.PageSize)
	}
	if cfg.InitialLoadTimeout != 5*time.Minute {
		t.Errorf("InitialLoadTimeout = %v, want %v", cfg.InitialLoadTimeout, 5*time.Minute)
	}
}

func TestNewRegistry(t *testing.T) {
	t.Run("with nil logger", func(t *testing.T) {
		cfg := DefaultConfig()
		client := api.NewClient("http://localhost", "")
		reg := NewRegistry(cfg, client, nil)
		if reg == nil {
			t.Fatal("NewRegistry returned nil")
		}
	})

	t.Run("with logger", func(t *testing.T) {
		cfg := DefaultConfig()
		client := api.NewClient("http://localhost", "")
		logger := slog.Default()
		reg := NewRegistry(cfg, client, logger)
		if reg == nil {
			t.Fatal("NewRegistry returned nil")
		}
	})
}

func TestRegistryImpl_GetActiveMarkets(t *testing.T) {
	cfg := DefaultConfig()
	client := api.NewClient("http://localhost", "")
	reg := NewRegistry(cfg, client, nil)

	// Cast to registryImpl for internal access
	impl := reg.(*registryImpl)
	impl.state.upsertMarket(model.Market{Ticker: "TEST-1", MarketStatus: "active"})
	impl.state.upsertMarket(model.Market{Ticker: "TEST-2", MarketStatus: "closed"})

	active := reg.GetActiveMarkets()
	if len(active) != 1 {
		t.Errorf("len(active) = %d, want 1", len(active))
	}
}

func TestRegistryImpl_GetMarket(t *testing.T) {
	cfg := DefaultConfig()
	client := api.NewClient("http://localhost", "")
	reg := NewRegistry(cfg, client, nil)

	impl := reg.(*registryImpl)
	impl.state.upsertMarket(model.Market{Ticker: "TEST-1", MarketStatus: "active"})

	t.Run("found", func(t *testing.T) {
		market, ok := reg.GetMarket("TEST-1")
		if !ok {
			t.Fatal("expected market to be found")
		}
		if market.Ticker != "TEST-1" {
			t.Errorf("Ticker = %q, want %q", market.Ticker, "TEST-1")
		}
	})

	t.Run("not found", func(t *testing.T) {
		_, ok := reg.GetMarket("NONEXISTENT")
		if ok {
			t.Error("expected market not to be found")
		}
	})
}

func TestRegistryImpl_SubscribeChanges(t *testing.T) {
	cfg := DefaultConfig()
	client := api.NewClient("http://localhost", "")
	reg := NewRegistry(cfg, client, nil)

	ch := reg.SubscribeChanges()
	if ch == nil {
		t.Fatal("SubscribeChanges returned nil")
	}
}

func TestRegistryImpl_SetLifecycleSource(t *testing.T) {
	cfg := DefaultConfig()
	client := api.NewClient("http://localhost", "")
	reg := NewRegistry(cfg, client, nil)

	ch := make(chan []byte)
	reg.SetLifecycleSource(ch)

	impl := reg.(*registryImpl)
	if impl.state.lifecycle != ch {
		t.Error("lifecycle channel not set correctly")
	}
}

func TestRegistryImpl_Stop_NilCancel(t *testing.T) {
	cfg := DefaultConfig()
	client := api.NewClient("http://localhost", "")
	reg := NewRegistry(cfg, client, nil)

	// Stop without Start should not panic
	ctx := context.Background()
	err := reg.Stop(ctx)
	if err != nil {
		t.Errorf("Stop returned error: %v", err)
	}
}

func TestRegistryImpl_StartAndStop(t *testing.T) {
	requestCount := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requestCount++
		if r.URL.Path == "/exchange/status" {
			json.NewEncoder(w).Encode(map[string]any{
				"exchange_active": true,
				"trading_active":  true,
			})
			return
		}
		// Return markets
		json.NewEncoder(w).Encode(map[string]any{
			"markets": []map[string]any{
				{"ticker": "MARKET-1", "status": "active"},
				{"ticker": "MARKET-2", "status": "closed"},
			},
			"cursor": "",
		})
	}))
	defer server.Close()

	cfg := Config{
		ReconcileInterval:  time.Hour, // Don't reconcile during test
		PageSize:           1000,
		InitialLoadTimeout: 5 * time.Minute,
	}
	client := api.NewClient(server.URL, "")
	reg := NewRegistry(cfg, client, nil)

	ctx := context.Background()
	err := reg.Start(ctx)
	if err != nil {
		t.Fatalf("Start failed: %v", err)
	}

	// Verify markets were loaded
	active := reg.GetActiveMarkets()
	if len(active) != 1 {
		t.Errorf("len(active) = %d, want 1", len(active))
	}

	stopCtx, cancel := context.WithTimeout(ctx, time.Second)
	defer cancel()

	err = reg.Stop(stopCtx)
	if err != nil {
		t.Fatalf("Stop failed: %v", err)
	}
}

func TestRegistryImpl_Start_ExchangeInactive(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/exchange/status" {
			json.NewEncoder(w).Encode(map[string]any{
				"exchange_active":       false,
				"trading_active":        false,
				"estimated_resume_time": "2024-01-15T10:00:00Z",
			})
			return
		}
		json.NewEncoder(w).Encode(map[string]any{
			"markets": []map[string]any{},
			"cursor":  "",
		})
	}))
	defer server.Close()

	cfg := DefaultConfig()
	client := api.NewClient(server.URL, "")
	reg := NewRegistry(cfg, client, nil)

	ctx := context.Background()
	err := reg.Start(ctx)
	// Should still start even if exchange is inactive
	if err != nil {
		t.Fatalf("Start failed: %v", err)
	}

	stopCtx, cancel := context.WithTimeout(ctx, time.Second)
	defer cancel()
	reg.Stop(stopCtx)
}

func TestConfig_ZeroValues(t *testing.T) {
	cfg := Config{}

	if cfg.ReconcileInterval != 0 {
		t.Errorf("ReconcileInterval = %v, want 0", cfg.ReconcileInterval)
	}
	if cfg.PageSize != 0 {
		t.Errorf("PageSize = %d, want 0", cfg.PageSize)
	}
	if cfg.InitialLoadTimeout != 0 {
		t.Errorf("InitialLoadTimeout = %v, want 0", cfg.InitialLoadTimeout)
	}
}

func TestRegistry_Interface(t *testing.T) {
	cfg := DefaultConfig()
	client := api.NewClient("http://localhost", "")
	reg := NewRegistry(cfg, client, nil)

	// Verify that registryImpl implements Registry interface
	var _ Registry = reg
}

// Tests for lifecycle message handling

func TestRegistryImpl_HandleLifecycleMessage_StatusChange(t *testing.T) {
	cfg := DefaultConfig()
	client := api.NewClient("http://localhost", "")
	reg := NewRegistry(cfg, client, nil)
	impl := reg.(*registryImpl)

	// Add a market first
	impl.state.upsertMarket(model.Market{
		Ticker:       "TEST-MARKET",
		MarketStatus: "open",
	})

	// Create lifecycle message for status change
	msg := `{
		"type": "market_lifecycle",
		"sid": 1,
		"msg": {
			"market_ticker": "TEST-MARKET",
			"event_type": "status_change",
			"old_status": "open",
			"new_status": "closed",
			"result": "",
			"ts": 1705328200
		}
	}`

	ctx := context.Background()
	impl.handleLifecycleMessage(ctx, []byte(msg))

	// Verify market status was updated
	market, ok := impl.GetMarket("TEST-MARKET")
	if !ok {
		t.Fatal("market not found")
	}
	if market.MarketStatus != "closed" {
		t.Errorf("MarketStatus = %q, want %q", market.MarketStatus, "closed")
	}

	// Verify market is no longer active
	active := impl.GetActiveMarkets()
	if len(active) != 0 {
		t.Errorf("expected no active markets after close")
	}
}

func TestRegistryImpl_HandleLifecycleMessage_Settled(t *testing.T) {
	cfg := DefaultConfig()
	client := api.NewClient("http://localhost", "")
	reg := NewRegistry(cfg, client, nil)
	impl := reg.(*registryImpl)

	// Add a market first
	impl.state.upsertMarket(model.Market{
		Ticker:       "TEST-MARKET",
		MarketStatus: "open",
	})

	// Create lifecycle message for settlement
	msg := `{
		"type": "market_lifecycle",
		"sid": 1,
		"msg": {
			"market_ticker": "TEST-MARKET",
			"event_type": "settled",
			"old_status": "open",
			"new_status": "settled",
			"result": "yes",
			"ts": 1705328200
		}
	}`

	ctx := context.Background()
	impl.handleLifecycleMessage(ctx, []byte(msg))

	// Verify market was settled
	market, ok := impl.GetMarket("TEST-MARKET")
	if !ok {
		t.Fatal("market not found")
	}
	if market.MarketStatus != "settled" {
		t.Errorf("MarketStatus = %q, want %q", market.MarketStatus, "settled")
	}
	if market.Result != "yes" {
		t.Errorf("Result = %q, want %q", market.Result, "yes")
	}
}

func TestRegistryImpl_HandleLifecycleMessage_InvalidJSON(t *testing.T) {
	cfg := DefaultConfig()
	client := api.NewClient("http://localhost", "")
	reg := NewRegistry(cfg, client, nil)
	impl := reg.(*registryImpl)

	// Invalid JSON should not panic
	ctx := context.Background()
	impl.handleLifecycleMessage(ctx, []byte("not json"))
	// No assertion - just verify it doesn't panic
}

func TestRegistryImpl_HandleLifecycleMessage_WrongType(t *testing.T) {
	cfg := DefaultConfig()
	client := api.NewClient("http://localhost", "")
	reg := NewRegistry(cfg, client, nil)
	impl := reg.(*registryImpl)

	// Message with wrong type should be ignored
	msg := `{
		"type": "ticker",
		"sid": 1,
		"msg": {}
	}`

	ctx := context.Background()
	impl.handleLifecycleMessage(ctx, []byte(msg))
	// No assertion - just verify it doesn't panic
}

func TestRegistryImpl_HandleStatusChange_UnknownMarket(t *testing.T) {
	cfg := DefaultConfig()
	client := api.NewClient("http://localhost", "")
	reg := NewRegistry(cfg, client, nil)
	impl := reg.(*registryImpl)

	// Status change for unknown market should log warning but not panic
	impl.handleStatusChange("UNKNOWN", "open", "closed")
	// No assertion - just verify it doesn't panic
}

func TestRegistryImpl_HandleSettled_UnknownMarket(t *testing.T) {
	cfg := DefaultConfig()
	client := api.NewClient("http://localhost", "")
	reg := NewRegistry(cfg, client, nil)
	impl := reg.(*registryImpl)

	// Settlement for unknown market should not panic
	impl.handleSettled("UNKNOWN", "yes")
	// No assertion - just verify it doesn't panic
}

func TestRegistryImpl_HandleMarketCreated(t *testing.T) {
	// Set up mock server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/markets/NEW-MARKET" {
			json.NewEncoder(w).Encode(map[string]any{
				"market": map[string]any{
					"ticker":       "NEW-MARKET",
					"event_ticker": "EVENT-1",
					"title":        "New Market",
					"status":       "active",
				},
			})
			return
		}
		w.WriteHeader(http.StatusNotFound)
	}))
	defer server.Close()

	cfg := DefaultConfig()
	client := api.NewClient(server.URL, "")
	reg := NewRegistry(cfg, client, nil)
	impl := reg.(*registryImpl)

	ctx := context.Background()
	impl.handleMarketCreated(ctx, "NEW-MARKET", "active")

	// Verify market was added
	market, ok := impl.GetMarket("NEW-MARKET")
	if !ok {
		t.Fatal("market not found")
	}
	if market.Ticker != "NEW-MARKET" {
		t.Errorf("Ticker = %q, want %q", market.Ticker, "NEW-MARKET")
	}
}

func TestRegistryImpl_HandleStatusChange_ToActive(t *testing.T) {
	cfg := DefaultConfig()
	client := api.NewClient("http://localhost", "")
	reg := NewRegistry(cfg, client, nil)
	impl := reg.(*registryImpl)

	// Add a closed market
	impl.state.upsertMarket(model.Market{
		Ticker:       "TEST-MARKET",
		MarketStatus: "closed",
	})

	// Verify not active initially
	active := impl.GetActiveMarkets()
	if len(active) != 0 {
		t.Errorf("expected no active markets initially")
	}

	// Handle status change to active
	impl.handleStatusChange("TEST-MARKET", "closed", "open")

	// Verify now active
	active = impl.GetActiveMarkets()
	if len(active) != 1 {
		t.Errorf("expected 1 active market after status change")
	}
}

func TestLifecycleLoop_ContextCancellation(t *testing.T) {
	cfg := DefaultConfig()
	client := api.NewClient("http://localhost", "")
	reg := NewRegistry(cfg, client, nil)
	impl := reg.(*registryImpl)

	// Set up lifecycle channel
	lifecycleCh := make(chan []byte, 1)
	impl.state.lifecycle = lifecycleCh

	// Create context that we'll cancel
	ctx, cancel := context.WithCancel(context.Background())

	// Run lifecycle loop in goroutine
	done := make(chan struct{})
	go func() {
		impl.lifecycleLoop(ctx)
		close(done)
	}()

	// Cancel context
	cancel()

	// Verify loop exits
	select {
	case <-done:
		// Success
	case <-time.After(time.Second):
		t.Error("lifecycle loop did not exit after context cancellation")
	}
}

func TestLifecycleLoop_ChannelClosed(t *testing.T) {
	cfg := DefaultConfig()
	client := api.NewClient("http://localhost", "")
	reg := NewRegistry(cfg, client, nil)
	impl := reg.(*registryImpl)

	// Set up lifecycle channel
	lifecycleCh := make(chan []byte, 1)
	impl.state.lifecycle = lifecycleCh

	ctx := context.Background()

	// Run lifecycle loop in goroutine
	done := make(chan struct{})
	go func() {
		impl.lifecycleLoop(ctx)
		close(done)
	}()

	// Close channel
	close(lifecycleCh)

	// Verify loop exits
	select {
	case <-done:
		// Success
	case <-time.After(time.Second):
		t.Error("lifecycle loop did not exit after channel closed")
	}
}
