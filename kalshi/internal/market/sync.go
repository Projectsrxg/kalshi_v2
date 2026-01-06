package market

import (
	"context"
	"encoding/json"
	"time"

	"github.com/rickgao/kalshi-data/internal/api"
)

// initialSync fetches active markets from REST API on startup.
// Fetches open and unopened markets, excluding settled/closed (1M+ historical).
func (r *registryImpl) initialSync(ctx context.Context) error {
	// Check exchange status first.
	if err := r.checkExchangeStatus(ctx); err != nil {
		return err
	}

	r.logger.Info("starting initial market sync (open + unopened markets)")
	start := time.Now()

	// Fetch open markets.
	r.logger.Info("fetching open markets")
	openMarkets, err := r.rest.GetAllMarketsWithOptions(ctx, api.GetMarketsOptions{
		Status: "open",
	})
	if err != nil {
		return err
	}
	r.logger.Info("fetched open markets", "count", len(openMarkets))

	// Fetch unopened markets.
	r.logger.Info("fetching unopened markets")
	unopenedMarkets, err := r.rest.GetAllMarketsWithOptions(ctx, api.GetMarketsOptions{
		Status: "unopened",
	})
	if err != nil {
		return err
	}
	r.logger.Info("fetched unopened markets", "count", len(unopenedMarkets))

	// Combine both lists.
	apiMarkets := make([]api.APIMarket, 0, len(openMarkets)+len(unopenedMarkets))
	apiMarkets = append(apiMarkets, openMarkets...)
	apiMarkets = append(apiMarkets, unopenedMarkets...)

	r.state.mu.Lock()
	for _, am := range apiMarkets {
		m := am.ToModel()
		r.state.upsertMarketLocked(m)

		if isActive(m.MarketStatus) {
			// Notify of new active market.
			r.state.notifyChange(MarketChange{
				Ticker:    m.Ticker,
				EventType: "created",
				NewStatus: m.MarketStatus,
				Market:    &m,
			})
		}
	}
	r.state.lastSyncAt = time.Now()
	r.state.mu.Unlock()

	r.logger.Info("initial sync complete",
		"total_markets", len(apiMarkets),
		"active_markets", len(r.state.activeSet),
		"duration", time.Since(start),
	)

	return nil
}

// checkExchangeStatus verifies the exchange is active.
func (r *registryImpl) checkExchangeStatus(ctx context.Context) error {
	status, err := r.rest.GetExchangeStatus(ctx)
	if err != nil {
		return err
	}

	r.state.mu.Lock()
	r.state.exchangeActive = status.ExchangeActive
	r.state.tradingActive = status.TradingActive
	r.state.mu.Unlock()

	if !status.ExchangeActive {
		r.logger.Warn("exchange is not active",
			"estimated_resume", status.EstimatedResumeTime,
		)
		// For now, continue anyway - reconciliation will retry.
	}

	return nil
}

// reconciliationLoop periodically syncs with REST API.
func (r *registryImpl) reconciliationLoop(ctx context.Context) {
	ticker := time.NewTicker(r.cfg.ReconcileInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			r.reconcile(ctx)
		}
	}
}

// reconcile fetches open and unopened markets and detects changes.
func (r *registryImpl) reconcile(ctx context.Context) {
	start := time.Now()

	// Fetch open markets.
	openMarkets, err := r.rest.GetAllMarketsWithOptions(ctx, api.GetMarketsOptions{
		Status: "open",
	})
	if err != nil {
		r.logger.Error("reconciliation failed fetching open markets", "err", err)
		return
	}

	// Fetch unopened markets.
	unopenedMarkets, err := r.rest.GetAllMarketsWithOptions(ctx, api.GetMarketsOptions{
		Status: "unopened",
	})
	if err != nil {
		r.logger.Error("reconciliation failed fetching unopened markets", "err", err)
		return
	}

	// Combine both lists.
	apiMarkets := make([]api.APIMarket, 0, len(openMarkets)+len(unopenedMarkets))
	apiMarkets = append(apiMarkets, openMarkets...)
	apiMarkets = append(apiMarkets, unopenedMarkets...)

	var created, changed int

	r.state.mu.Lock()
	for _, am := range apiMarkets {
		m := am.ToModel()
		existing, ok := r.state.markets[m.Ticker]

		if !ok {
			// New market we missed.
			r.state.upsertMarketLocked(m)
			if isActive(m.MarketStatus) {
				r.state.notifyChange(MarketChange{
					Ticker:    m.Ticker,
					EventType: "created",
					NewStatus: m.MarketStatus,
					Market:    &m,
				})
				created++
			}
			continue
		}

		// Check for status changes we missed.
		if existing.MarketStatus != m.MarketStatus {
			oldStatus := existing.MarketStatus
			r.state.upsertMarketLocked(m)

			r.state.notifyChange(MarketChange{
				Ticker:    m.Ticker,
				EventType: "status_change",
				OldStatus: oldStatus,
				NewStatus: m.MarketStatus,
				Market:    &m,
			})
			changed++
		}
	}
	r.state.lastSyncAt = time.Now()
	r.state.mu.Unlock()

	if created > 0 || changed > 0 {
		r.logger.Info("reconciliation found changes",
			"created", created,
			"changed", changed,
			"duration", time.Since(start),
		)
	} else {
		r.logger.Debug("reconciliation complete",
			"total_markets", len(apiMarkets),
			"duration", time.Since(start),
		)
	}
}

// lifecycleLoop processes WebSocket lifecycle events.
func (r *registryImpl) lifecycleLoop(ctx context.Context) {
	for {
		select {
		case <-ctx.Done():
			return
		case msg, ok := <-r.state.lifecycle:
			if !ok {
				return
			}
			r.handleLifecycleMessage(ctx, msg)
		}
	}
}

// lifecycleMessage is the WebSocket message for market_lifecycle channel.
type lifecycleMessage struct {
	Type string `json:"type"`
	SID  int64  `json:"sid"`
	Msg  struct {
		MarketTicker string `json:"market_ticker"`
		EventType    string `json:"event_type"` // "created", "status_change", "settled"
		OldStatus    string `json:"old_status"`
		NewStatus    string `json:"new_status"`
		Result       string `json:"result"` // "yes", "no", or ""
		Timestamp    int64  `json:"ts"`     // Unix timestamp (seconds)
	} `json:"msg"`
}

// handleLifecycleMessage processes a single lifecycle message from WebSocket.
func (r *registryImpl) handleLifecycleMessage(ctx context.Context, msg []byte) {
	var lm lifecycleMessage
	if err := json.Unmarshal(msg, &lm); err != nil {
		r.logger.Warn("failed to parse lifecycle message", "error", err)
		return
	}

	// Only process market_lifecycle messages
	if lm.Type != "market_lifecycle" {
		return
	}

	ticker := lm.Msg.MarketTicker
	eventType := lm.Msg.EventType

	r.logger.Debug("processing lifecycle event",
		"ticker", ticker,
		"event", eventType,
		"old_status", lm.Msg.OldStatus,
		"new_status", lm.Msg.NewStatus,
	)

	switch eventType {
	case "created":
		// New market created - fetch full details from REST API
		r.handleMarketCreated(ctx, ticker, lm.Msg.NewStatus)

	case "status_change":
		// Market status changed
		r.handleStatusChange(ticker, lm.Msg.OldStatus, lm.Msg.NewStatus)

	case "settled":
		// Market settled
		r.handleSettled(ticker, lm.Msg.Result)

	default:
		r.logger.Warn("unknown lifecycle event type", "type", eventType)
	}
}

// handleMarketCreated handles a new market being created.
func (r *registryImpl) handleMarketCreated(ctx context.Context, ticker, status string) {
	// Fetch full market details from REST API
	apiMarket, err := r.fetchMarket(ctx, ticker)
	if err != nil {
		r.logger.Warn("failed to fetch new market", "ticker", ticker, "error", err)
		return
	}

	m := apiMarket.ToModel()

	r.state.mu.Lock()
	r.state.upsertMarketLocked(m)
	r.state.mu.Unlock()

	// Notify if active
	if isActive(m.MarketStatus) {
		r.state.notifyChange(MarketChange{
			Ticker:    ticker,
			EventType: "created",
			NewStatus: m.MarketStatus,
			Market:    &m,
		})
	}

	r.logger.Info("market created via lifecycle",
		"ticker", ticker,
		"status", m.MarketStatus,
	)
}

// handleStatusChange handles a market status change.
func (r *registryImpl) handleStatusChange(ticker, oldStatus, newStatus string) {
	r.state.mu.Lock()
	existing, ok := r.state.markets[ticker]
	if !ok {
		r.state.mu.Unlock()
		r.logger.Warn("status change for unknown market", "ticker", ticker)
		return
	}

	// Update status
	existing.MarketStatus = newStatus

	// Update active set
	wasActive := isActive(oldStatus)
	nowActive := isActive(newStatus)

	if nowActive && !wasActive {
		r.state.activeSet[ticker] = struct{}{}
	} else if !nowActive && wasActive {
		delete(r.state.activeSet, ticker)
	}

	// Copy for notification (while holding lock)
	marketCopy := *existing
	r.state.mu.Unlock()

	// Notify of change
	r.state.notifyChange(MarketChange{
		Ticker:    ticker,
		EventType: "status_change",
		OldStatus: oldStatus,
		NewStatus: newStatus,
		Market:    &marketCopy,
	})

	r.logger.Debug("market status changed",
		"ticker", ticker,
		"old", oldStatus,
		"new", newStatus,
	)
}

// handleSettled handles a market being settled.
func (r *registryImpl) handleSettled(ticker, result string) {
	r.state.mu.Lock()
	existing, ok := r.state.markets[ticker]
	if ok {
		existing.MarketStatus = "settled"
		existing.Result = result
		delete(r.state.activeSet, ticker)
	}
	r.state.mu.Unlock()

	if !ok {
		return
	}

	// Notify of settlement
	r.state.notifyChange(MarketChange{
		Ticker:    ticker,
		EventType: "settled",
		OldStatus: "open",
		NewStatus: "settled",
		Market:    nil, // Market no longer active
	})

	r.logger.Debug("market settled",
		"ticker", ticker,
		"result", result,
	)
}

// fetchMarket fetches a single market from REST API.
func (r *registryImpl) fetchMarket(ctx context.Context, ticker string) (*api.APIMarket, error) {
	return r.rest.GetMarket(ctx, ticker)
}
