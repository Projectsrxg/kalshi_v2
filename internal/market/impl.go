package market

import (
	"context"
	"log/slog"
	"sync"
	"time"

	"github.com/rickgao/kalshi-data/internal/api"
	"github.com/rickgao/kalshi-data/internal/model"
)

// Config holds Market Registry configuration.
type Config struct {
	ReconcileInterval  time.Duration
	PageSize           int
	InitialLoadTimeout time.Duration
}

// DefaultConfig returns sensible defaults.
func DefaultConfig() Config {
	return Config{
		ReconcileInterval:  5 * time.Minute,
		PageSize:           1000,
		InitialLoadTimeout: 5 * time.Minute,
	}
}

// registryImpl implements the Registry interface.
type registryImpl struct {
	cfg    Config
	rest   *api.Client
	logger *slog.Logger

	state *registryState

	ctx    context.Context
	cancel context.CancelFunc
	wg     sync.WaitGroup
}

// NewRegistry creates a new Market Registry.
func NewRegistry(cfg Config, rest *api.Client, logger *slog.Logger) Registry {
	if logger == nil {
		logger = slog.Default()
	}

	return &registryImpl{
		cfg:    cfg,
		rest:   rest,
		logger: logger,
		state:  newState(),
	}
}

// Start begins market discovery in the background.
func (r *registryImpl) Start(ctx context.Context) error {
	r.ctx, r.cancel = context.WithCancel(ctx)

	// Initial sync (blocking).
	if err := r.initialSync(r.ctx); err != nil {
		r.cancel()
		return err
	}

	// Start background reconciliation.
	r.wg.Add(1)
	go func() {
		defer r.wg.Done()
		r.reconciliationLoop(r.ctx)
	}()

	// Start lifecycle handler if source is set.
	if r.state.lifecycle != nil {
		r.wg.Add(1)
		go func() {
			defer r.wg.Done()
			r.lifecycleLoop(r.ctx)
		}()
	}

	r.logger.Info("market registry started",
		"active_markets", len(r.state.activeSet),
		"total_markets", len(r.state.markets),
	)

	return nil
}

// Stop gracefully shuts down.
func (r *registryImpl) Stop(ctx context.Context) error {
	if r.cancel != nil {
		r.cancel()
	}

	done := make(chan struct{})
	go func() {
		r.wg.Wait()
		close(done)
	}()

	select {
	case <-done:
		r.logger.Info("market registry stopped")
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

// GetActiveMarkets returns all markets currently open for trading.
func (r *registryImpl) GetActiveMarkets() []model.Market {
	return r.state.getActiveMarkets()
}

// GetMarket returns a specific market by ticker.
func (r *registryImpl) GetMarket(ticker string) (model.Market, bool) {
	return r.state.getMarket(ticker)
}

// SubscribeChanges returns a channel of market state changes.
func (r *registryImpl) SubscribeChanges() <-chan MarketChange {
	return r.state.changes
}

// SetLifecycleSource sets the channel for lifecycle messages.
func (r *registryImpl) SetLifecycleSource(ch <-chan []byte) {
	r.state.lifecycle = ch
}
