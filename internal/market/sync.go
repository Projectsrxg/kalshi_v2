package market

import (
	"context"
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

// handleLifecycleMessage processes a single lifecycle message.
// For now, this is a placeholder - WebSocket parsing will be added later.
func (r *registryImpl) handleLifecycleMessage(ctx context.Context, msg []byte) {
	// TODO: Parse WebSocket message and handle lifecycle events.
	// For now, we rely on reconciliation to catch changes.
	r.logger.Debug("received lifecycle message", "len", len(msg))
}

// fetchMarket fetches a single market from REST API.
func (r *registryImpl) fetchMarket(ctx context.Context, ticker string) (*api.APIMarket, error) {
	return r.rest.GetMarket(ctx, ticker)
}
