package market

import (
	"sync"
	"time"

	"github.com/rickgao/kalshi-data/internal/model"
)

// registryState holds the thread-safe market cache.
type registryState struct {
	mu sync.RWMutex

	// All known markets indexed by ticker.
	markets map[string]*model.Market

	// Markets currently active (open for trading).
	activeSet map[string]struct{}

	// Exchange status.
	exchangeActive bool
	tradingActive  bool

	// Last successful REST sync timestamp.
	lastSyncAt time.Time

	// Output channel for Connection Manager.
	changes chan MarketChange

	// Input channel from Connection Manager (market_lifecycle messages).
	lifecycle <-chan []byte
}

func newState() *registryState {
	return &registryState{
		markets:   make(map[string]*model.Market),
		activeSet: make(map[string]struct{}),
		changes:   make(chan MarketChange, ChangeBufferSize),
	}
}

// getMarket returns a market by ticker (read-locked).
func (s *registryState) getMarket(ticker string) (model.Market, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	m, ok := s.markets[ticker]
	if !ok {
		return model.Market{}, false
	}
	return *m, true
}

// getActiveMarkets returns a copy of all active markets (read-locked).
func (s *registryState) getActiveMarkets() []model.Market {
	s.mu.RLock()
	defer s.mu.RUnlock()

	result := make([]model.Market, 0, len(s.activeSet))
	for ticker := range s.activeSet {
		if m, ok := s.markets[ticker]; ok {
			result = append(result, *m)
		}
	}
	return result
}

// upsertMarket adds or updates a market (write-locked).
func (s *registryState) upsertMarket(m model.Market) {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.upsertMarketLocked(m)
}

// upsertMarketLocked adds or updates a market (caller must hold write lock).
func (s *registryState) upsertMarketLocked(m model.Market) {
	mCopy := m
	s.markets[m.Ticker] = &mCopy

	if isActive(m.MarketStatus) {
		s.activeSet[m.Ticker] = struct{}{}
	} else {
		delete(s.activeSet, m.Ticker)
	}
}

// updateStatus updates a market's status (write-locked).
func (s *registryState) updateStatus(ticker, newStatus string) (oldStatus string, found bool) {
	s.mu.Lock()
	defer s.mu.Unlock()

	m, ok := s.markets[ticker]
	if !ok {
		return "", false
	}

	oldStatus = m.MarketStatus
	m.MarketStatus = newStatus

	if isActive(newStatus) {
		s.activeSet[ticker] = struct{}{}
	} else {
		delete(s.activeSet, ticker)
	}

	return oldStatus, true
}

// notifyChange sends a change to the changes channel (non-blocking).
func (s *registryState) notifyChange(change MarketChange) {
	select {
	case s.changes <- change:
	default:
		// Channel full, drop oldest by consuming one and retrying.
		select {
		case <-s.changes:
			s.changes <- change
		default:
		}
	}
}

// isActive returns true if the status means the market is tradeable.
func isActive(status string) bool {
	return status == "active" || status == "open"
}
