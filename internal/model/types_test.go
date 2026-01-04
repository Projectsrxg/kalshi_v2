package model

import (
	"testing"

	"github.com/google/uuid"
)

// TestModelTypes validates that model types can be instantiated correctly.
func TestModelTypes(t *testing.T) {
	t.Run("Series", func(t *testing.T) {
		s := Series{
			Ticker:            "TEST-SERIES",
			Title:             "Test Series",
			Category:          "Politics",
			Frequency:         "daily",
			Tags:              map[string]string{"key": "value"},
			SettlementSources: []string{"AP", "Reuters"},
			UpdatedAt:         1705321845000000,
		}

		if s.Ticker != "TEST-SERIES" {
			t.Errorf("Ticker = %q, want %q", s.Ticker, "TEST-SERIES")
		}
		if s.UpdatedAt != 1705321845000000 {
			t.Errorf("UpdatedAt = %d, want %d", s.UpdatedAt, 1705321845000000)
		}
	})

	t.Run("Event", func(t *testing.T) {
		e := Event{
			EventTicker:  "TEST-EVENT",
			SeriesTicker: "TEST-SERIES",
			Title:        "Test Event",
			Category:     "Politics",
			SubTitle:     "Subtitle",
			CreatedTS:    1705321845000000,
			UpdatedAt:    1705321845000000,
		}

		if e.EventTicker != "TEST-EVENT" {
			t.Errorf("EventTicker = %q, want %q", e.EventTicker, "TEST-EVENT")
		}
		if e.SeriesTicker != "TEST-SERIES" {
			t.Errorf("SeriesTicker = %q, want %q", e.SeriesTicker, "TEST-SERIES")
		}
	})

	t.Run("Market", func(t *testing.T) {
		m := Market{
			Ticker:        "TEST-MARKET",
			EventTicker:   "TEST-EVENT",
			Title:         "Test Market",
			Subtitle:      "Subtitle",
			MarketStatus:  "open",
			TradingStatus: "active",
			MarketType:    "binary",
			Result:        "",
			YesBid:        52000,
			YesAsk:        54000,
			LastPrice:     53000,
			Volume:        1000,
			Volume24h:     500,
			OpenInterest:  200,
			OpenTS:        1704067200000000,
			CloseTS:       1735689599000000,
			ExpirationTS:  1735776000000000,
			CreatedTS:     1701388800000000,
			UpdatedAt:     1705321845000000,
		}

		if m.Ticker != "TEST-MARKET" {
			t.Errorf("Ticker = %q, want %q", m.Ticker, "TEST-MARKET")
		}
		if m.YesBid != 52000 {
			t.Errorf("YesBid = %d, want %d", m.YesBid, 52000)
		}
		if m.MarketType != "binary" {
			t.Errorf("MarketType = %q, want %q", m.MarketType, "binary")
		}
	})

	t.Run("Trade", func(t *testing.T) {
		tradeID := uuid.New()
		tr := Trade{
			TradeID:    tradeID,
			ExchangeTS: 1705321845000000,
			ReceivedAt: 1705321845100000,
			Ticker:     "TEST-MARKET",
			Price:      52000,
			Size:       10,
			TakerSide:  true,
		}

		if tr.TradeID != tradeID {
			t.Errorf("TradeID = %v, want %v", tr.TradeID, tradeID)
		}
		if tr.Price != 52000 {
			t.Errorf("Price = %d, want %d", tr.Price, 52000)
		}
		if !tr.TakerSide {
			t.Error("TakerSide = false, want true")
		}
	})

	t.Run("OrderbookDelta", func(t *testing.T) {
		d := OrderbookDelta{
			ExchangeTS: 1705321845000000,
			ReceivedAt: 1705321845100000,
			Ticker:     "TEST-MARKET",
			Side:       true,
			Price:      52000,
			SizeDelta:  100,
			Seq:        12345,
		}

		if d.Ticker != "TEST-MARKET" {
			t.Errorf("Ticker = %q, want %q", d.Ticker, "TEST-MARKET")
		}
		if d.SizeDelta != 100 {
			t.Errorf("SizeDelta = %d, want %d", d.SizeDelta, 100)
		}
		if !d.Side {
			t.Error("Side = false, want true (YES)")
		}
	})

	t.Run("PriceLevel", func(t *testing.T) {
		p := PriceLevel{
			Price: 52000,
			Size:  100,
		}

		if p.Price != 52000 {
			t.Errorf("Price = %d, want %d", p.Price, 52000)
		}
		if p.Size != 100 {
			t.Errorf("Size = %d, want %d", p.Size, 100)
		}
	})

	t.Run("OrderbookSnapshot", func(t *testing.T) {
		s := OrderbookSnapshot{
			SnapshotTS: 1705321845000000,
			ExchangeTS: 0,
			Ticker:     "TEST-MARKET",
			Source:     "rest",
			YesBids:    []PriceLevel{{Price: 52000, Size: 100}},
			YesAsks:    []PriceLevel{{Price: 54000, Size: 50}},
			NoBids:     []PriceLevel{{Price: 48000, Size: 75}},
			NoAsks:     []PriceLevel{{Price: 46000, Size: 25}},
			BestYesBid: 52000,
			BestYesAsk: 54000,
			Spread:     2000,
		}

		if s.Ticker != "TEST-MARKET" {
			t.Errorf("Ticker = %q, want %q", s.Ticker, "TEST-MARKET")
		}
		if s.Source != "rest" {
			t.Errorf("Source = %q, want %q", s.Source, "rest")
		}
		if len(s.YesBids) != 1 {
			t.Errorf("len(YesBids) = %d, want 1", len(s.YesBids))
		}
		if s.Spread != 2000 {
			t.Errorf("Spread = %d, want %d", s.Spread, 2000)
		}
	})

	t.Run("Ticker", func(t *testing.T) {
		tk := Ticker{
			ExchangeTS:         1705321845000000,
			ReceivedAt:         1705321845100000,
			Ticker:             "TEST-MARKET",
			YesBid:             52000,
			YesAsk:             54000,
			LastPrice:          53000,
			Volume:             1000,
			OpenInterest:       200,
			DollarVolume:       50000,
			DollarOpenInterest: 10000,
		}

		if tk.Ticker != "TEST-MARKET" {
			t.Errorf("Ticker = %q, want %q", tk.Ticker, "TEST-MARKET")
		}
		if tk.YesBid != 52000 {
			t.Errorf("YesBid = %d, want %d", tk.YesBid, 52000)
		}
		if tk.DollarVolume != 50000 {
			t.Errorf("DollarVolume = %d, want %d", tk.DollarVolume, 50000)
		}
	})
}

// TestZeroValues tests that zero values are handled correctly.
func TestZeroValues(t *testing.T) {
	t.Run("zero value Market", func(t *testing.T) {
		var m Market
		if m.Ticker != "" {
			t.Errorf("zero Market.Ticker = %q, want empty", m.Ticker)
		}
		if m.YesBid != 0 {
			t.Errorf("zero Market.YesBid = %d, want 0", m.YesBid)
		}
	})

	t.Run("zero value Trade", func(t *testing.T) {
		var tr Trade
		if tr.TradeID != uuid.Nil {
			t.Errorf("zero Trade.TradeID = %v, want nil UUID", tr.TradeID)
		}
		if tr.TakerSide != false {
			t.Error("zero Trade.TakerSide = true, want false")
		}
	})
}

// TestPriceRanges tests price values at boundaries.
func TestPriceRanges(t *testing.T) {
	tests := []struct {
		name     string
		price    int
		isValid  bool
		expected string
	}{
		{"zero", 0, true, "0.00000"},
		{"one cent", 1000, true, "0.01000"},
		{"fifty cents", 50000, true, "0.50000"},
		{"one dollar", 100000, true, "1.00000"},
		{"sub-penny", 52505, true, "0.52505"},
		{"max precision", 99999, true, "0.99999"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m := Market{
				Ticker: "PRICE-TEST",
				YesBid: tt.price,
			}
			if m.YesBid != tt.price {
				t.Errorf("YesBid = %d, want %d", m.YesBid, tt.price)
			}
		})
	}
}
