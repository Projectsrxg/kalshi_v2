package api

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

// TestNewClient tests client construction with various options.
func TestNewClient(t *testing.T) {
	t.Run("default values", func(t *testing.T) {
		c := NewClient("https://api.example.com", "test-key")

		if c.baseURL != "https://api.example.com" {
			t.Errorf("baseURL = %q, want %q", c.baseURL, "https://api.example.com")
		}
		if c.apiKey != "test-key" {
			t.Errorf("apiKey = %q, want %q", c.apiKey, "test-key")
		}
		if c.httpClient.Timeout != 30*time.Second {
			t.Errorf("Timeout = %v, want %v", c.httpClient.Timeout, 30*time.Second)
		}
		if c.maxRetries != 3 {
			t.Errorf("maxRetries = %d, want %d", c.maxRetries, 3)
		}
		if c.retryBackoff != time.Second {
			t.Errorf("retryBackoff = %v, want %v", c.retryBackoff, time.Second)
		}
		if c.logger == nil {
			t.Error("logger should not be nil")
		}
	})

	t.Run("with timeout option", func(t *testing.T) {
		c := NewClient("https://api.example.com", "", WithTimeout(5*time.Second))
		if c.httpClient.Timeout != 5*time.Second {
			t.Errorf("Timeout = %v, want %v", c.httpClient.Timeout, 5*time.Second)
		}
	})

	t.Run("with retries option", func(t *testing.T) {
		c := NewClient("https://api.example.com", "", WithRetries(5, 2*time.Second))
		if c.maxRetries != 5 {
			t.Errorf("maxRetries = %d, want %d", c.maxRetries, 5)
		}
		if c.retryBackoff != 2*time.Second {
			t.Errorf("retryBackoff = %v, want %v", c.retryBackoff, 2*time.Second)
		}
	})

	t.Run("with logger option", func(t *testing.T) {
		logger := slog.New(slog.NewTextHandler(os.Stderr, nil))
		c := NewClient("https://api.example.com", "", WithLogger(logger))
		if c.logger != logger {
			t.Error("logger not set correctly")
		}
	})

	t.Run("with custom HTTP client", func(t *testing.T) {
		customClient := &http.Client{Timeout: 10 * time.Second}
		c := NewClient("https://api.example.com", "", WithHTTPClient(customClient))
		if c.httpClient != customClient {
			t.Error("custom HTTP client not set")
		}
	})

	t.Run("with multiple options", func(t *testing.T) {
		logger := slog.New(slog.NewTextHandler(os.Stderr, nil))
		c := NewClient("https://api.example.com", "key",
			WithTimeout(15*time.Second),
			WithRetries(10, 500*time.Millisecond),
			WithLogger(logger),
		)
		if c.httpClient.Timeout != 15*time.Second {
			t.Errorf("Timeout = %v, want %v", c.httpClient.Timeout, 15*time.Second)
		}
		if c.maxRetries != 10 {
			t.Errorf("maxRetries = %d, want %d", c.maxRetries, 10)
		}
		if c.retryBackoff != 500*time.Millisecond {
			t.Errorf("retryBackoff = %v, want %v", c.retryBackoff, 500*time.Millisecond)
		}
		if c.logger != logger {
			t.Error("logger not set correctly")
		}
	})

	t.Run("empty API key", func(t *testing.T) {
		c := NewClient("https://api.example.com", "")
		if c.apiKey != "" {
			t.Errorf("apiKey = %q, want empty", c.apiKey)
		}
	})
}

// TestAPIError tests the APIError type.
func TestAPIError(t *testing.T) {
	t.Run("Error method", func(t *testing.T) {
		err := &APIError{
			StatusCode: 404,
			Message:    "Not Found",
			Body:       []byte(`{"error": "market not found"}`),
		}
		expected := "kalshi api error 404: Not Found"
		if err.Error() != expected {
			t.Errorf("Error() = %q, want %q", err.Error(), expected)
		}
	})

	t.Run("IsRetryable for 5xx errors", func(t *testing.T) {
		tests := []struct {
			code     int
			expected bool
		}{
			{500, true},
			{502, true},
			{503, true},
			{504, true},
			{429, true},
			{400, false},
			{401, false},
			{403, false},
			{404, false},
			{200, false},
			{499, false},
		}

		for _, tt := range tests {
			err := &APIError{StatusCode: tt.code}
			if got := err.IsRetryable(); got != tt.expected {
				t.Errorf("IsRetryable() for status %d = %v, want %v", tt.code, got, tt.expected)
			}
		}
	})
}

// TestDoRequest tests the HTTP request functionality.
func TestDoRequest(t *testing.T) {
	t.Run("successful request", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.Header.Get("Accept") != "application/json" {
				t.Errorf("Accept header = %q, want %q", r.Header.Get("Accept"), "application/json")
			}
			if r.Header.Get("Authorization") != "Bearer test-key" {
				t.Errorf("Authorization header = %q, want %q", r.Header.Get("Authorization"), "Bearer test-key")
			}
			w.WriteHeader(http.StatusOK)
			w.Write([]byte(`{"status": "ok"}`))
		}))
		defer server.Close()

		c := NewClient(server.URL, "test-key")
		body, err := c.doRequest(context.Background(), http.MethodGet, "/test", nil)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if string(body) != `{"status": "ok"}` {
			t.Errorf("body = %q, want %q", string(body), `{"status": "ok"}`)
		}
	})

	t.Run("request without API key", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.Header.Get("Authorization") != "" {
				t.Errorf("Authorization header should be empty, got %q", r.Header.Get("Authorization"))
			}
			w.WriteHeader(http.StatusOK)
			w.Write([]byte(`{}`))
		}))
		defer server.Close()

		c := NewClient(server.URL, "")
		_, err := c.doRequest(context.Background(), http.MethodGet, "/test", nil)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
	})

	t.Run("request with query parameters", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.URL.Query().Get("limit") != "10" {
				t.Errorf("limit = %q, want %q", r.URL.Query().Get("limit"), "10")
			}
			if r.URL.Query().Get("cursor") != "abc123" {
				t.Errorf("cursor = %q, want %q", r.URL.Query().Get("cursor"), "abc123")
			}
			w.WriteHeader(http.StatusOK)
			w.Write([]byte(`{}`))
		}))
		defer server.Close()

		c := NewClient(server.URL, "key")
		query := make(map[string][]string)
		query["limit"] = []string{"10"}
		query["cursor"] = []string{"abc123"}
		_, err := c.doRequest(context.Background(), http.MethodGet, "/test", query)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
	})

	t.Run("4xx error returns APIError", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusNotFound)
			w.Write([]byte(`{"error": "not found"}`))
		}))
		defer server.Close()

		c := NewClient(server.URL, "key")
		_, err := c.doRequest(context.Background(), http.MethodGet, "/test", nil)
		if err == nil {
			t.Fatal("expected error, got nil")
		}

		apiErr, ok := err.(*APIError)
		if !ok {
			t.Fatalf("expected *APIError, got %T", err)
		}
		if apiErr.StatusCode != 404 {
			t.Errorf("StatusCode = %d, want %d", apiErr.StatusCode, 404)
		}
		if !strings.Contains(string(apiErr.Body), "not found") {
			t.Errorf("Body should contain 'not found', got %q", string(apiErr.Body))
		}
	})

	t.Run("5xx error returns APIError", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusInternalServerError)
			w.Write([]byte(`internal error`))
		}))
		defer server.Close()

		c := NewClient(server.URL, "key")
		_, err := c.doRequest(context.Background(), http.MethodGet, "/test", nil)
		if err == nil {
			t.Fatal("expected error, got nil")
		}

		apiErr, ok := err.(*APIError)
		if !ok {
			t.Fatalf("expected *APIError, got %T", err)
		}
		if apiErr.StatusCode != 500 {
			t.Errorf("StatusCode = %d, want %d", apiErr.StatusCode, 500)
		}
	})

	t.Run("context cancellation", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			time.Sleep(100 * time.Millisecond)
			w.WriteHeader(http.StatusOK)
		}))
		defer server.Close()

		c := NewClient(server.URL, "key")
		ctx, cancel := context.WithCancel(context.Background())
		cancel() // Cancel immediately

		_, err := c.doRequest(ctx, http.MethodGet, "/test", nil)
		if err == nil {
			t.Fatal("expected error, got nil")
		}
		if !strings.Contains(err.Error(), "context canceled") {
			t.Errorf("error should contain 'context canceled', got %v", err)
		}
	})
}

// TestDoWithRetry tests the retry logic.
func TestDoWithRetry(t *testing.T) {
	t.Run("succeeds on first try", func(t *testing.T) {
		var attempts int32
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			atomic.AddInt32(&attempts, 1)
			w.WriteHeader(http.StatusOK)
			w.Write([]byte(`{"ok": true}`))
		}))
		defer server.Close()

		c := NewClient(server.URL, "key", WithRetries(3, 10*time.Millisecond))
		body, err := c.doWithRetry(context.Background(), http.MethodGet, "/test", nil)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if string(body) != `{"ok": true}` {
			t.Errorf("body = %q, want %q", string(body), `{"ok": true}`)
		}
		if attempts != 1 {
			t.Errorf("attempts = %d, want 1", attempts)
		}
	})

	t.Run("retries on 5xx and succeeds", func(t *testing.T) {
		var attempts int32
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			n := atomic.AddInt32(&attempts, 1)
			if n < 3 {
				w.WriteHeader(http.StatusInternalServerError)
				w.Write([]byte(`error`))
				return
			}
			w.WriteHeader(http.StatusOK)
			w.Write([]byte(`{"ok": true}`))
		}))
		defer server.Close()

		c := NewClient(server.URL, "key", WithRetries(3, 10*time.Millisecond))
		body, err := c.doWithRetry(context.Background(), http.MethodGet, "/test", nil)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if string(body) != `{"ok": true}` {
			t.Errorf("body = %q, want %q", string(body), `{"ok": true}`)
		}
		if attempts != 3 {
			t.Errorf("attempts = %d, want 3", attempts)
		}
	})

	t.Run("retries on 429 and succeeds", func(t *testing.T) {
		var attempts int32
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			n := atomic.AddInt32(&attempts, 1)
			if n == 1 {
				w.WriteHeader(http.StatusTooManyRequests)
				w.Write([]byte(`rate limited`))
				return
			}
			w.WriteHeader(http.StatusOK)
			w.Write([]byte(`{"ok": true}`))
		}))
		defer server.Close()

		c := NewClient(server.URL, "key", WithRetries(3, 10*time.Millisecond))
		_, err := c.doWithRetry(context.Background(), http.MethodGet, "/test", nil)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if attempts != 2 {
			t.Errorf("attempts = %d, want 2", attempts)
		}
	})

	t.Run("does not retry on 4xx (except 429)", func(t *testing.T) {
		var attempts int32
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			atomic.AddInt32(&attempts, 1)
			w.WriteHeader(http.StatusBadRequest)
			w.Write([]byte(`bad request`))
		}))
		defer server.Close()

		c := NewClient(server.URL, "key", WithRetries(3, 10*time.Millisecond))
		_, err := c.doWithRetry(context.Background(), http.MethodGet, "/test", nil)
		if err == nil {
			t.Fatal("expected error, got nil")
		}
		if attempts != 1 {
			t.Errorf("attempts = %d, want 1", attempts)
		}
	})

	t.Run("max retries exceeded", func(t *testing.T) {
		var attempts int32
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			atomic.AddInt32(&attempts, 1)
			w.WriteHeader(http.StatusInternalServerError)
			w.Write([]byte(`error`))
		}))
		defer server.Close()

		c := NewClient(server.URL, "key", WithRetries(2, 10*time.Millisecond))
		_, err := c.doWithRetry(context.Background(), http.MethodGet, "/test", nil)
		if err == nil {
			t.Fatal("expected error, got nil")
		}
		if !strings.Contains(err.Error(), "max retries exceeded") {
			t.Errorf("error should contain 'max retries exceeded', got %v", err)
		}
		// 1 initial + 2 retries = 3 attempts
		if attempts != 3 {
			t.Errorf("attempts = %d, want 3", attempts)
		}
	})

	t.Run("context cancellation during retry", func(t *testing.T) {
		var attempts int32
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			atomic.AddInt32(&attempts, 1)
			w.WriteHeader(http.StatusInternalServerError)
		}))
		defer server.Close()

		c := NewClient(server.URL, "key", WithRetries(5, 50*time.Millisecond))
		ctx, cancel := context.WithTimeout(context.Background(), 80*time.Millisecond)
		defer cancel()

		_, err := c.doWithRetry(ctx, http.MethodGet, "/test", nil)
		if err == nil {
			t.Fatal("expected error, got nil")
		}
		if !strings.Contains(err.Error(), "context") {
			t.Errorf("error should be context-related, got %v", err)
		}
	})
}

// TestGetExchangeStatus tests the GetExchangeStatus method.
func TestGetExchangeStatus(t *testing.T) {
	t.Run("successful response", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.URL.Path != "/exchange/status" {
				t.Errorf("path = %q, want %q", r.URL.Path, "/exchange/status")
			}
			w.WriteHeader(http.StatusOK)
			json.NewEncoder(w).Encode(ExchangeStatusResponse{
				ExchangeActive: true,
				TradingActive:  true,
			})
		}))
		defer server.Close()

		c := NewClient(server.URL, "key")
		status, err := c.GetExchangeStatus(context.Background())
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if !status.ExchangeActive {
			t.Error("ExchangeActive = false, want true")
		}
		if !status.TradingActive {
			t.Error("TradingActive = false, want true")
		}
	})

	t.Run("exchange inactive with resume time", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusOK)
			json.NewEncoder(w).Encode(ExchangeStatusResponse{
				ExchangeActive:      false,
				TradingActive:       false,
				EstimatedResumeTime: "2024-01-15T10:00:00Z",
			})
		}))
		defer server.Close()

		c := NewClient(server.URL, "key")
		status, err := c.GetExchangeStatus(context.Background())
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if status.ExchangeActive {
			t.Error("ExchangeActive = true, want false")
		}
		if status.EstimatedResumeTime != "2024-01-15T10:00:00Z" {
			t.Errorf("EstimatedResumeTime = %q, want %q", status.EstimatedResumeTime, "2024-01-15T10:00:00Z")
		}
	})

	t.Run("error response", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusServiceUnavailable)
		}))
		defer server.Close()

		c := NewClient(server.URL, "key", WithRetries(0, time.Millisecond))
		_, err := c.GetExchangeStatus(context.Background())
		if err == nil {
			t.Fatal("expected error, got nil")
		}
	})
}

// TestGetMarkets tests the GetMarkets method.
func TestGetMarkets(t *testing.T) {
	t.Run("basic request", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.URL.Path != "/markets" {
				t.Errorf("path = %q, want %q", r.URL.Path, "/markets")
			}
			w.WriteHeader(http.StatusOK)
			json.NewEncoder(w).Encode(MarketsResponse{
				Markets: []APIMarket{
					{Ticker: "MKT1", Title: "Market 1"},
					{Ticker: "MKT2", Title: "Market 2"},
				},
				Cursor: "",
			})
		}))
		defer server.Close()

		c := NewClient(server.URL, "key")
		resp, err := c.GetMarkets(context.Background(), GetMarketsOptions{})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(resp.Markets) != 2 {
			t.Errorf("len(Markets) = %d, want 2", len(resp.Markets))
		}
		if resp.Markets[0].Ticker != "MKT1" {
			t.Errorf("Markets[0].Ticker = %q, want %q", resp.Markets[0].Ticker, "MKT1")
		}
	})

	t.Run("with options", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			q := r.URL.Query()
			if q.Get("limit") != "100" {
				t.Errorf("limit = %q, want %q", q.Get("limit"), "100")
			}
			if q.Get("cursor") != "cursor123" {
				t.Errorf("cursor = %q, want %q", q.Get("cursor"), "cursor123")
			}
			if q.Get("event_ticker") != "EVENT1" {
				t.Errorf("event_ticker = %q, want %q", q.Get("event_ticker"), "EVENT1")
			}
			if q.Get("series_ticker") != "SERIES1" {
				t.Errorf("series_ticker = %q, want %q", q.Get("series_ticker"), "SERIES1")
			}
			if q.Get("status") != "open" {
				t.Errorf("status = %q, want %q", q.Get("status"), "open")
			}
			if q.Get("tickers") != "MKT1,MKT2" {
				t.Errorf("tickers = %q, want %q", q.Get("tickers"), "MKT1,MKT2")
			}
			w.WriteHeader(http.StatusOK)
			json.NewEncoder(w).Encode(MarketsResponse{Markets: []APIMarket{}, Cursor: ""})
		}))
		defer server.Close()

		c := NewClient(server.URL, "key")
		_, err := c.GetMarkets(context.Background(), GetMarketsOptions{
			Limit:        100,
			Cursor:       "cursor123",
			EventTicker:  "EVENT1",
			SeriesTicker: "SERIES1",
			Tickers:      []string{"MKT1", "MKT2"},
			Status:       "open",
		})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
	})

	t.Run("with cursor for pagination", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusOK)
			json.NewEncoder(w).Encode(MarketsResponse{
				Markets: []APIMarket{{Ticker: "MKT1"}},
				Cursor:  "next_page",
			})
		}))
		defer server.Close()

		c := NewClient(server.URL, "key")
		resp, err := c.GetMarkets(context.Background(), GetMarketsOptions{})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if resp.Cursor != "next_page" {
			t.Errorf("Cursor = %q, want %q", resp.Cursor, "next_page")
		}
	})
}

// TestGetAllMarkets tests pagination through all markets.
func TestGetAllMarkets(t *testing.T) {
	t.Run("single page", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusOK)
			json.NewEncoder(w).Encode(MarketsResponse{
				Markets: []APIMarket{{Ticker: "MKT1"}, {Ticker: "MKT2"}},
				Cursor:  "",
			})
		}))
		defer server.Close()

		c := NewClient(server.URL, "key")
		markets, err := c.GetAllMarkets(context.Background())
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(markets) != 2 {
			t.Errorf("len(markets) = %d, want 2", len(markets))
		}
	})

	t.Run("multiple pages", func(t *testing.T) {
		var requestCount int32
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			count := atomic.AddInt32(&requestCount, 1)
			cursor := r.URL.Query().Get("cursor")

			switch {
			case count == 1 && cursor == "":
				json.NewEncoder(w).Encode(MarketsResponse{
					Markets: []APIMarket{{Ticker: "MKT1"}, {Ticker: "MKT2"}},
					Cursor:  "page2",
				})
			case count == 2 && cursor == "page2":
				json.NewEncoder(w).Encode(MarketsResponse{
					Markets: []APIMarket{{Ticker: "MKT3"}},
					Cursor:  "page3",
				})
			case count == 3 && cursor == "page3":
				json.NewEncoder(w).Encode(MarketsResponse{
					Markets: []APIMarket{{Ticker: "MKT4"}},
					Cursor:  "",
				})
			default:
				t.Errorf("unexpected request: count=%d cursor=%q", count, cursor)
			}
		}))
		defer server.Close()

		c := NewClient(server.URL, "key")
		markets, err := c.GetAllMarkets(context.Background())
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(markets) != 4 {
			t.Errorf("len(markets) = %d, want 4", len(markets))
		}
		if requestCount != 3 {
			t.Errorf("requestCount = %d, want 3", requestCount)
		}
	})

	t.Run("applies default pagination timeout", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			json.NewEncoder(w).Encode(MarketsResponse{Markets: []APIMarket{}, Cursor: ""})
		}))
		defer server.Close()

		c := NewClient(server.URL, "key")
		// Pass context without deadline - should apply DefaultPaginationTimeout
		ctx := context.Background()
		_, err := c.GetAllMarkets(ctx)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
	})

	t.Run("respects existing context deadline", func(t *testing.T) {
		requestCh := make(chan struct{}, 1)
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			requestCh <- struct{}{}
			time.Sleep(100 * time.Millisecond)
			json.NewEncoder(w).Encode(MarketsResponse{Markets: []APIMarket{}, Cursor: ""})
		}))
		defer server.Close()

		c := NewClient(server.URL, "key")
		ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
		defer cancel()

		_, err := c.GetAllMarkets(ctx)
		if err == nil {
			t.Fatal("expected timeout error")
		}
		<-requestCh // Ensure request was made
	})
}

// TestGetAllMarketsWithOptions tests filtered pagination.
func TestGetAllMarketsWithOptions(t *testing.T) {
	t.Run("with event ticker filter", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.URL.Query().Get("event_ticker") != "EVENT1" {
				t.Errorf("event_ticker = %q, want %q", r.URL.Query().Get("event_ticker"), "EVENT1")
			}
			if r.URL.Query().Get("limit") != "1000" {
				t.Errorf("limit = %q, want %q", r.URL.Query().Get("limit"), "1000")
			}
			json.NewEncoder(w).Encode(MarketsResponse{
				Markets: []APIMarket{{Ticker: "MKT1", EventTicker: "EVENT1"}},
				Cursor:  "",
			})
		}))
		defer server.Close()

		c := NewClient(server.URL, "key")
		markets, err := c.GetAllMarketsWithOptions(context.Background(), GetMarketsOptions{
			EventTicker: "EVENT1",
		})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(markets) != 1 {
			t.Errorf("len(markets) = %d, want 1", len(markets))
		}
	})
}

// TestGetMarket tests fetching a single market.
func TestGetMarket(t *testing.T) {
	t.Run("successful fetch", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.URL.Path != "/markets/TEST-MARKET" {
				t.Errorf("path = %q, want %q", r.URL.Path, "/markets/TEST-MARKET")
			}
			json.NewEncoder(w).Encode(SingleMarketResponse{
				Market: APIMarket{
					Ticker:        "TEST-MARKET",
					Title:         "Test Market",
					Status:        "open",
					YesBidDollars: "0.52",
					YesAskDollars: "0.54",
				},
			})
		}))
		defer server.Close()

		c := NewClient(server.URL, "key")
		market, err := c.GetMarket(context.Background(), "TEST-MARKET")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if market.Ticker != "TEST-MARKET" {
			t.Errorf("Ticker = %q, want %q", market.Ticker, "TEST-MARKET")
		}
		if market.Status != "open" {
			t.Errorf("Status = %q, want %q", market.Status, "open")
		}
	})

	t.Run("not found", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusNotFound)
			json.NewEncoder(w).Encode(map[string]string{"error": "market not found"})
		}))
		defer server.Close()

		c := NewClient(server.URL, "key", WithRetries(0, time.Millisecond))
		_, err := c.GetMarket(context.Background(), "NONEXISTENT")
		if err == nil {
			t.Fatal("expected error, got nil")
		}
		var apiErr *APIError
		if !errors.As(err, &apiErr) {
			t.Fatalf("expected *APIError in wrapped error, got %T: %v", err, err)
		}
		if apiErr.StatusCode != 404 {
			t.Errorf("StatusCode = %d, want 404", apiErr.StatusCode)
		}
	})
}

// TestGetOrderbook tests fetching orderbook data.
func TestGetOrderbook(t *testing.T) {
	t.Run("successful fetch", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.URL.Path != "/markets/TEST-MARKET/orderbook" {
				t.Errorf("path = %q, want %q", r.URL.Path, "/markets/TEST-MARKET/orderbook")
			}
			json.NewEncoder(w).Encode(OrderbookResponse{
				Orderbook: APIOrderbook{
					Yes: [][]int{{52, 100}, {51, 200}},
					No:  [][]int{{48, 150}},
				},
			})
		}))
		defer server.Close()

		c := NewClient(server.URL, "key")
		ob, err := c.GetOrderbook(context.Background(), "TEST-MARKET", 0)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(ob.Orderbook.Yes) != 2 {
			t.Errorf("len(Yes) = %d, want 2", len(ob.Orderbook.Yes))
		}
		if len(ob.Orderbook.No) != 1 {
			t.Errorf("len(No) = %d, want 1", len(ob.Orderbook.No))
		}
	})

	t.Run("with depth parameter", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.URL.Query().Get("depth") != "5" {
				t.Errorf("depth = %q, want %q", r.URL.Query().Get("depth"), "5")
			}
			json.NewEncoder(w).Encode(OrderbookResponse{Orderbook: APIOrderbook{}})
		}))
		defer server.Close()

		c := NewClient(server.URL, "key")
		_, err := c.GetOrderbook(context.Background(), "TEST-MARKET", 5)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
	})

	t.Run("depth 0 does not send parameter", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.URL.Query().Has("depth") {
				t.Errorf("depth parameter should not be set")
			}
			json.NewEncoder(w).Encode(OrderbookResponse{Orderbook: APIOrderbook{}})
		}))
		defer server.Close()

		c := NewClient(server.URL, "key")
		_, err := c.GetOrderbook(context.Background(), "TEST-MARKET", 0)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
	})
}

// TestGetEvents tests the GetEvents method.
func TestGetEvents(t *testing.T) {
	t.Run("basic request", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.URL.Path != "/events" {
				t.Errorf("path = %q, want %q", r.URL.Path, "/events")
			}
			json.NewEncoder(w).Encode(EventsResponse{
				Events: []APIEvent{
					{EventTicker: "EVT1", Title: "Event 1"},
					{EventTicker: "EVT2", Title: "Event 2"},
				},
				Cursor: "",
			})
		}))
		defer server.Close()

		c := NewClient(server.URL, "key")
		resp, err := c.GetEvents(context.Background(), GetEventsOptions{})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(resp.Events) != 2 {
			t.Errorf("len(Events) = %d, want 2", len(resp.Events))
		}
	})

	t.Run("with options", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			q := r.URL.Query()
			if q.Get("limit") != "50" {
				t.Errorf("limit = %q, want %q", q.Get("limit"), "50")
			}
			if q.Get("cursor") != "cursor456" {
				t.Errorf("cursor = %q, want %q", q.Get("cursor"), "cursor456")
			}
			if q.Get("series_ticker") != "SERIES1" {
				t.Errorf("series_ticker = %q, want %q", q.Get("series_ticker"), "SERIES1")
			}
			if q.Get("status") != "open" {
				t.Errorf("status = %q, want %q", q.Get("status"), "open")
			}
			json.NewEncoder(w).Encode(EventsResponse{Events: []APIEvent{}, Cursor: ""})
		}))
		defer server.Close()

		c := NewClient(server.URL, "key")
		_, err := c.GetEvents(context.Background(), GetEventsOptions{
			Limit:        50,
			Cursor:       "cursor456",
			SeriesTicker: "SERIES1",
			Status:       "open",
		})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
	})
}

// TestGetAllEvents tests pagination through all events.
func TestGetAllEvents(t *testing.T) {
	t.Run("multiple pages", func(t *testing.T) {
		var requestCount int32
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			count := atomic.AddInt32(&requestCount, 1)
			cursor := r.URL.Query().Get("cursor")

			if count == 1 && cursor == "" {
				json.NewEncoder(w).Encode(EventsResponse{
					Events: []APIEvent{{EventTicker: "EVT1"}},
					Cursor: "page2",
				})
			} else if count == 2 && cursor == "page2" {
				json.NewEncoder(w).Encode(EventsResponse{
					Events: []APIEvent{{EventTicker: "EVT2"}},
					Cursor: "",
				})
			}
		}))
		defer server.Close()

		c := NewClient(server.URL, "key")
		events, err := c.GetAllEvents(context.Background())
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(events) != 2 {
			t.Errorf("len(events) = %d, want 2", len(events))
		}
		if requestCount != 2 {
			t.Errorf("requestCount = %d, want 2", requestCount)
		}
	})
}

// TestGetEvent tests fetching a single event.
func TestGetEvent(t *testing.T) {
	t.Run("successful fetch", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.URL.Path != "/events/TEST-EVENT" {
				t.Errorf("path = %q, want %q", r.URL.Path, "/events/TEST-EVENT")
			}
			json.NewEncoder(w).Encode(SingleEventResponse{
				Event: APIEvent{
					EventTicker:  "TEST-EVENT",
					SeriesTicker: "SERIES1",
					Title:        "Test Event",
				},
			})
		}))
		defer server.Close()

		c := NewClient(server.URL, "key")
		event, err := c.GetEvent(context.Background(), "TEST-EVENT")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if event.EventTicker != "TEST-EVENT" {
			t.Errorf("EventTicker = %q, want %q", event.EventTicker, "TEST-EVENT")
		}
	})
}

// TestGetSeries tests fetching a series.
func TestGetSeries(t *testing.T) {
	t.Run("successful fetch", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.URL.Path != "/series/TEST-SERIES" {
				t.Errorf("path = %q, want %q", r.URL.Path, "/series/TEST-SERIES")
			}
			json.NewEncoder(w).Encode(SeriesResponse{
				Series: APISeries{
					Ticker:   "TEST-SERIES",
					Title:    "Test Series",
					Category: "Politics",
					Tags:     []string{"tag1", "tag2"},
				},
			})
		}))
		defer server.Close()

		c := NewClient(server.URL, "key")
		series, err := c.GetSeries(context.Background(), "TEST-SERIES")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if series.Ticker != "TEST-SERIES" {
			t.Errorf("Ticker = %q, want %q", series.Ticker, "TEST-SERIES")
		}
		if len(series.Tags) != 2 {
			t.Errorf("len(Tags) = %d, want 2", len(series.Tags))
		}
	})
}

// TestJSONUnmarshalErrors tests error handling for invalid JSON.
func TestJSONUnmarshalErrors(t *testing.T) {
	t.Run("invalid JSON response", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusOK)
			w.Write([]byte(`not valid json`))
		}))
		defer server.Close()

		c := NewClient(server.URL, "key")
		_, err := c.GetExchangeStatus(context.Background())
		if err == nil {
			t.Fatal("expected error, got nil")
		}
		if !strings.Contains(err.Error(), "unmarshal") {
			t.Errorf("error should contain 'unmarshal', got %v", err)
		}
	})
}
