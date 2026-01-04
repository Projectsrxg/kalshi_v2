package market

import (
	"testing"

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
		{"", false},
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
