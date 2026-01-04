package poller

import (
	"context"
	"log/slog"
	"sync"
	"sync/atomic"
	"time"

	"github.com/rickgao/kalshi-data/internal/api"
	"github.com/rickgao/kalshi-data/internal/model"
)

// MarketSource provides active markets to poll.
type MarketSource interface {
	GetActiveMarkets() []model.Market
}

// SnapshotHandler receives fetched snapshots.
type SnapshotHandler interface {
	HandleSnapshot(snapshot model.OrderbookSnapshot) error
}

// SnapshotHandlerFunc is a function adapter for SnapshotHandler.
type SnapshotHandlerFunc func(model.OrderbookSnapshot) error

func (f SnapshotHandlerFunc) HandleSnapshot(s model.OrderbookSnapshot) error {
	return f(s)
}

// Config holds poller configuration.
type Config struct {
	Interval    time.Duration // Poll interval (default: 15m)
	Concurrency int           // Max concurrent requests (default: 100)
	Timeout     time.Duration // Per-request timeout (default: 10s)
}

// DefaultConfig returns sensible defaults.
func DefaultConfig() Config {
	return Config{
		Interval:    15 * time.Minute,
		Concurrency: 100,
		Timeout:     10 * time.Second,
	}
}

// Poller periodically fetches orderbook snapshots via REST API.
type Poller struct {
	cfg     Config
	client  *api.Client
	markets MarketSource
	handler SnapshotHandler
	logger  *slog.Logger

	ctx    context.Context
	cancel context.CancelFunc
	wg     sync.WaitGroup
}

// New creates a new Poller.
func New(cfg Config, client *api.Client, markets MarketSource, handler SnapshotHandler, logger *slog.Logger) *Poller {
	if logger == nil {
		logger = slog.Default()
	}
	return &Poller{
		cfg:     cfg,
		client:  client,
		markets: markets,
		handler: handler,
		logger:  logger,
	}
}

// Start begins the polling loop.
func (p *Poller) Start(ctx context.Context) error {
	p.ctx, p.cancel = context.WithCancel(ctx)

	p.wg.Add(1)
	go p.run()

	p.logger.Info("snapshot poller started",
		"interval", p.cfg.Interval,
		"concurrency", p.cfg.Concurrency,
	)

	return nil
}

// Stop gracefully shuts down the poller.
func (p *Poller) Stop(ctx context.Context) error {
	if p.cancel != nil {
		p.cancel()
	}

	done := make(chan struct{})
	go func() {
		p.wg.Wait()
		close(done)
	}()

	select {
	case <-done:
		p.logger.Info("snapshot poller stopped")
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

// run is the main polling loop.
func (p *Poller) run() {
	defer p.wg.Done()

	ticker := time.NewTicker(p.cfg.Interval)
	defer ticker.Stop()

	// Poll immediately on start.
	p.pollAll()

	for {
		select {
		case <-p.ctx.Done():
			return
		case <-ticker.C:
			p.pollAll()
		}
	}
}

// pollAll fetches orderbooks for all active markets concurrently.
func (p *Poller) pollAll() {
	start := time.Now()

	markets := p.markets.GetActiveMarkets()
	if len(markets) == 0 {
		p.logger.Debug("no active markets to poll")
		return
	}

	// Semaphore for bounded concurrency.
	sem := make(chan struct{}, p.cfg.Concurrency)
	var wg sync.WaitGroup
	var fetched, errors atomic.Int64

	for _, market := range markets {
		wg.Add(1)
		go func(ticker string) {
			defer wg.Done()

			// Acquire semaphore slot.
			select {
			case sem <- struct{}{}:
				defer func() { <-sem }()
			case <-p.ctx.Done():
				return
			}

			if err := p.pollMarket(ticker); err != nil {
				p.logger.Warn("failed to poll market",
					"ticker", ticker,
					"err", err,
				)
				errors.Add(1)
				return
			}

			fetched.Add(1)
		}(market.Ticker)
	}

	wg.Wait()

	p.logger.Info("poll cycle complete",
		"markets", len(markets),
		"fetched", fetched.Load(),
		"errors", errors.Load(),
		"duration", time.Since(start),
	)
}

// pollMarket fetches and handles a single market's orderbook.
func (p *Poller) pollMarket(ticker string) error {
	ctx, cancel := context.WithTimeout(p.ctx, p.cfg.Timeout)
	defer cancel()

	ob, err := p.client.GetOrderbook(ctx, ticker, 0) // 0 = all levels
	if err != nil {
		return err
	}

	snapshot := ob.ToOrderbookSnapshot(ticker, "rest")

	if p.handler != nil {
		if err := p.handler.HandleSnapshot(snapshot); err != nil {
			return err
		}
	}

	return nil
}
