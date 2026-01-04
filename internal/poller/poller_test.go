package poller

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"

	"github.com/rickgao/kalshi-data/internal/api"
	"github.com/rickgao/kalshi-data/internal/model"
)

// mockMarketSource returns a fixed list of markets.
type mockMarketSource struct {
	markets []model.Market
}

func (m *mockMarketSource) GetActiveMarkets() []model.Market {
	return m.markets
}

func TestPoller_PollAll(t *testing.T) {
	// Create a test server that returns orderbook data.
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		resp := map[string]any{
			"orderbook": map[string]any{
				"yes": [][]int{{52, 100}, {51, 200}},
				"no":  [][]int{{48, 150}},
			},
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	client := api.NewClient(server.URL, "", api.WithTimeout(5*time.Second))

	markets := &mockMarketSource{
		markets: []model.Market{
			{Ticker: "MARKET-1", MarketStatus: "active"},
			{Ticker: "MARKET-2", MarketStatus: "active"},
			{Ticker: "MARKET-3", MarketStatus: "active"},
		},
	}

	var snapshotCount atomic.Int32
	handler := SnapshotHandlerFunc(func(s model.OrderbookSnapshot) error {
		snapshotCount.Add(1)
		return nil
	})

	cfg := Config{
		Interval:    time.Hour, // Long interval, we'll trigger manually.
		Concurrency: 10,
		Timeout:     5 * time.Second,
	}

	p := New(cfg, client, markets, handler, nil)

	// Call pollAll directly.
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	p.ctx = ctx

	p.pollAll()

	if got := snapshotCount.Load(); got != 3 {
		t.Errorf("snapshotCount = %d, want 3", got)
	}
}

func TestPoller_StartStop(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		resp := map[string]any{
			"orderbook": map[string]any{
				"yes": [][]int{},
				"no":  [][]int{},
			},
		}
		json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	client := api.NewClient(server.URL, "")

	markets := &mockMarketSource{
		markets: []model.Market{
			{Ticker: "TEST-1", MarketStatus: "active"},
		},
	}

	var called atomic.Bool
	handler := SnapshotHandlerFunc(func(s model.OrderbookSnapshot) error {
		called.Store(true)
		return nil
	})

	cfg := Config{
		Interval:    100 * time.Millisecond,
		Concurrency: 10,
		Timeout:     5 * time.Second,
	}

	p := New(cfg, client, markets, handler, nil)

	ctx := context.Background()
	if err := p.Start(ctx); err != nil {
		t.Fatalf("Start failed: %v", err)
	}

	// Wait for at least one poll.
	time.Sleep(150 * time.Millisecond)

	stopCtx, cancel := context.WithTimeout(ctx, time.Second)
	defer cancel()

	if err := p.Stop(stopCtx); err != nil {
		t.Fatalf("Stop failed: %v", err)
	}

	if !called.Load() {
		t.Error("handler was never called")
	}
}

func TestPoller_Concurrency(t *testing.T) {
	var inFlight atomic.Int32
	var maxInFlight atomic.Int32

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		current := inFlight.Add(1)
		defer inFlight.Add(-1)

		// Track max concurrent requests.
		for {
			old := maxInFlight.Load()
			if current <= old || maxInFlight.CompareAndSwap(old, current) {
				break
			}
		}

		// Simulate some work.
		time.Sleep(50 * time.Millisecond)

		resp := map[string]any{
			"orderbook": map[string]any{
				"yes": [][]int{},
				"no":  [][]int{},
			},
		}
		json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	client := api.NewClient(server.URL, "")

	// Create 20 markets.
	var marketList []model.Market
	for i := 0; i < 20; i++ {
		marketList = append(marketList, model.Market{
			Ticker:       "MARKET-" + string(rune('A'+i)),
			MarketStatus: "active",
		})
	}
	markets := &mockMarketSource{markets: marketList}

	handler := SnapshotHandlerFunc(func(s model.OrderbookSnapshot) error {
		return nil
	})

	cfg := Config{
		Interval:    time.Hour,
		Concurrency: 5, // Limit to 5 concurrent.
		Timeout:     5 * time.Second,
	}

	p := New(cfg, client, markets, handler, nil)

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	p.ctx = ctx

	p.pollAll()

	if got := maxInFlight.Load(); got > 5 {
		t.Errorf("maxInFlight = %d, want <= 5", got)
	}
}
