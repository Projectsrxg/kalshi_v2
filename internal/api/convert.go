package api

import (
	"strconv"
	"strings"
	"time"

	"github.com/rickgao/kalshi-data/internal/model"
)

// DollarsToInternal converts a dollar string to internal representation.
// "0.52" -> 52000, "0.5250" -> 52500, "0.52505" -> 52505
// Returns 0 for empty or invalid input.
func DollarsToInternal(dollars string) int {
	if dollars == "" {
		return 0
	}

	// Remove leading zeros and parse
	dollars = strings.TrimSpace(dollars)

	f, err := strconv.ParseFloat(dollars, 64)
	if err != nil {
		return 0
	}

	// Multiply by 100,000 and round to int
	return int(f*100000 + 0.5)
}

// CentsToInternal converts cents (int) to internal representation.
// 52 cents -> 52000 internal
func CentsToInternal(cents int) int {
	return cents * 1000
}

// ParseTimestamp parses an ISO 8601 timestamp to microseconds since epoch.
// Returns 0 for empty or invalid input.
func ParseTimestamp(iso string) int64 {
	if iso == "" {
		return 0
	}

	t, err := time.Parse(time.RFC3339, iso)
	if err != nil {
		// Try without timezone
		t, err = time.Parse("2006-01-02T15:04:05", iso)
		if err != nil {
			return 0
		}
	}

	return t.UnixMicro()
}

// NowMicro returns the current time in microseconds since epoch.
func NowMicro() int64 {
	return time.Now().UnixMicro()
}

// ToModel converts an APIMarket to model.Market.
func (m *APIMarket) ToModel() model.Market {
	return model.Market{
		Ticker:        m.Ticker,
		EventTicker:   m.EventTicker,
		Title:         m.Title,
		Subtitle:      m.Subtitle,
		MarketStatus:  m.Status,
		TradingStatus: m.Status, // Same as status for now
		MarketType:    m.MarketType,
		Result:        m.Result,
		YesBid:        DollarsToInternal(m.YesBidDollars),
		YesAsk:        DollarsToInternal(m.YesAskDollars),
		LastPrice:     DollarsToInternal(m.LastPriceDollars),
		Volume:        m.Volume,
		Volume24h:     m.Volume24h,
		OpenInterest:  m.OpenInterest,
		OpenTS:        ParseTimestamp(m.OpenTime),
		CloseTS:       ParseTimestamp(m.CloseTime),
		ExpirationTS:  ParseTimestamp(m.ExpirationTime),
		CreatedTS:     ParseTimestamp(m.CreatedTime),
		UpdatedAt:     NowMicro(),
	}
}

// ToModel converts an APIEvent to model.Event.
func (e *APIEvent) ToModel() model.Event {
	return model.Event{
		EventTicker:  e.EventTicker,
		SeriesTicker: e.SeriesTicker,
		Title:        e.Title,
		SubTitle:     e.Subtitle,
		Category:     e.Category,
		UpdatedAt:    NowMicro(),
	}
}

// ToModel converts an APISeries to model.Series.
func (s *APISeries) ToModel() model.Series {
	tags := make(map[string]string)
	for _, tag := range s.Tags {
		tags[tag] = "true"
	}

	return model.Series{
		Ticker:            s.Ticker,
		Title:             s.Title,
		Category:          s.Category,
		Frequency:         s.Frequency,
		Tags:              tags,
		SettlementSources: s.SettlementSources,
		UpdatedAt:         NowMicro(),
	}
}

// ToOrderbookSnapshot converts an OrderbookResponse to model.OrderbookSnapshot.
func (o *OrderbookResponse) ToOrderbookSnapshot(ticker string, source string) model.OrderbookSnapshot {
	now := NowMicro()

	yesBids := make([]model.PriceLevel, 0, len(o.Orderbook.Yes))
	for _, level := range o.Orderbook.Yes {
		if len(level) >= 2 {
			yesBids = append(yesBids, model.PriceLevel{
				Price: CentsToInternal(level[0]),
				Size:  level[1],
			})
		}
	}

	noBids := make([]model.PriceLevel, 0, len(o.Orderbook.No))
	for _, level := range o.Orderbook.No {
		if len(level) >= 2 {
			noBids = append(noBids, model.PriceLevel{
				Price: CentsToInternal(level[0]),
				Size:  level[1],
			})
		}
	}

	var bestYesBid, bestYesAsk int
	if len(yesBids) > 0 {
		bestYesBid = yesBids[0].Price
	}
	if len(noBids) > 0 {
		// Best YES ask = 100000 - best NO bid
		bestYesAsk = 100000 - noBids[0].Price
	}

	spread := 0
	if bestYesBid > 0 && bestYesAsk > 0 {
		spread = bestYesAsk - bestYesBid
	}

	return model.OrderbookSnapshot{
		SnapshotTS: now,
		ExchangeTS: 0, // Not provided by REST API
		Ticker:     ticker,
		Source:     source,
		YesBids:    yesBids,
		NoBids:     noBids,
		BestYesBid: bestYesBid,
		BestYesAsk: bestYesAsk,
		Spread:     spread,
	}
}
