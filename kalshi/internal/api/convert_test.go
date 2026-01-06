package api

import (
	"testing"
	"time"
)

func TestDollarsToInternal(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  int
	}{
		{"standard penny", "0.52", 52000},
		{"sub-penny 1 digit", "0.5250", 52500},
		{"sub-penny 2 digits", "0.52505", 52505},
		{"zero", "0.00", 0},
		{"one dollar", "1.00", 100000},
		{"max precision", "0.99999", 99999},
		{"one cent", "0.01", 1000},
		{"empty string", "", 0},
		{"invalid string", "invalid", 0},
		{"whitespace trimmed", "  0.52  ", 52000},
		{"very small", "0.00001", 1},
		{"half cent", "0.005", 500},
		{"near zero", "0.001", 100},
		{"full dollar", "1.0", 100000},
		{"leading zero", "0.05", 5000},
		{"multiple decimal places", "0.123456", 12346}, // Rounds to nearest
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := DollarsToInternal(tt.input)
			if got != tt.want {
				t.Errorf("DollarsToInternal(%q) = %d, want %d", tt.input, got, tt.want)
			}
		})
	}
}

func TestCentsToInternal(t *testing.T) {
	tests := []struct {
		name  string
		input int
		want  int
	}{
		{"standard", 52, 52000},
		{"zero", 0, 0},
		{"full dollar", 100, 100000},
		{"one cent", 1, 1000},
		{"fifty cents", 50, 50000},
		{"ninety-nine cents", 99, 99000},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := CentsToInternal(tt.input)
			if got != tt.want {
				t.Errorf("CentsToInternal(%d) = %d, want %d", tt.input, got, tt.want)
			}
		})
	}
}

func TestParseTimestamp(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  int64
	}{
		{"empty", "", 0},
		{"invalid", "invalid", 0},
		{"RFC3339 UTC", "2024-01-15T12:30:45Z", 1705321845000000},
		{"RFC3339 with offset", "2024-01-15T12:30:45+00:00", 1705321845000000},
		{"without timezone", "2024-01-15T12:30:45", 1705321845000000},
		{"epoch", "1970-01-01T00:00:00Z", 0},
		{"year 2000", "2000-01-01T00:00:00Z", 946684800000000},
		{"with milliseconds", "2024-01-15T12:30:45.123Z", 1705321845123000},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := ParseTimestamp(tt.input)
			if got != tt.want {
				t.Errorf("ParseTimestamp(%q) = %d, want %d", tt.input, got, tt.want)
			}
		})
	}
}

func TestNowMicro(t *testing.T) {
	before := time.Now().UnixMicro()
	got := NowMicro()
	after := time.Now().UnixMicro()

	if got < before || got > after {
		t.Errorf("NowMicro() = %d, not between %d and %d", got, before, after)
	}
}

func TestAPIMarketToModel(t *testing.T) {
	t.Run("full market conversion", func(t *testing.T) {
		m := APIMarket{
			Ticker:           "TEST-MARKET",
			EventTicker:      "TEST-EVENT",
			Title:            "Test Market",
			Subtitle:         "Test subtitle",
			Status:           "open",
			MarketType:       "binary",
			Result:           "",
			YesBidDollars:    "0.52",
			YesAskDollars:    "0.54",
			LastPriceDollars: "0.53",
			Volume:           1000,
			Volume24h:        500,
			OpenInterest:     200,
			OpenTime:         "2024-01-01T00:00:00Z",
			CloseTime:        "2024-12-31T23:59:59Z",
			ExpirationTime:   "2025-01-01T00:00:00Z",
			CreatedTime:      "2023-12-01T00:00:00Z",
		}

		model := m.ToModel()

		if model.Ticker != "TEST-MARKET" {
			t.Errorf("Ticker = %q, want %q", model.Ticker, "TEST-MARKET")
		}
		if model.EventTicker != "TEST-EVENT" {
			t.Errorf("EventTicker = %q, want %q", model.EventTicker, "TEST-EVENT")
		}
		if model.Title != "Test Market" {
			t.Errorf("Title = %q, want %q", model.Title, "Test Market")
		}
		if model.Subtitle != "Test subtitle" {
			t.Errorf("Subtitle = %q, want %q", model.Subtitle, "Test subtitle")
		}
		if model.MarketStatus != "open" {
			t.Errorf("MarketStatus = %q, want %q", model.MarketStatus, "open")
		}
		if model.TradingStatus != "open" {
			t.Errorf("TradingStatus = %q, want %q", model.TradingStatus, "open")
		}
		if model.MarketType != "binary" {
			t.Errorf("MarketType = %q, want %q", model.MarketType, "binary")
		}
		if model.YesBid != 52000 {
			t.Errorf("YesBid = %d, want %d", model.YesBid, 52000)
		}
		if model.YesAsk != 54000 {
			t.Errorf("YesAsk = %d, want %d", model.YesAsk, 54000)
		}
		if model.LastPrice != 53000 {
			t.Errorf("LastPrice = %d, want %d", model.LastPrice, 53000)
		}
		if model.Volume != 1000 {
			t.Errorf("Volume = %d, want %d", model.Volume, 1000)
		}
		if model.Volume24h != 500 {
			t.Errorf("Volume24h = %d, want %d", model.Volume24h, 500)
		}
		if model.OpenInterest != 200 {
			t.Errorf("OpenInterest = %d, want %d", model.OpenInterest, 200)
		}
		if model.OpenTS == 0 {
			t.Error("OpenTS should not be zero")
		}
		if model.CloseTS == 0 {
			t.Error("CloseTS should not be zero")
		}
		if model.ExpirationTS == 0 {
			t.Error("ExpirationTS should not be zero")
		}
		if model.CreatedTS == 0 {
			t.Error("CreatedTS should not be zero")
		}
		if model.UpdatedAt == 0 {
			t.Error("UpdatedAt should not be zero")
		}
	})

	t.Run("market with empty prices", func(t *testing.T) {
		m := APIMarket{
			Ticker:           "EMPTY-MARKET",
			YesBidDollars:    "",
			YesAskDollars:    "",
			LastPriceDollars: "",
		}

		model := m.ToModel()

		if model.YesBid != 0 {
			t.Errorf("YesBid = %d, want 0", model.YesBid)
		}
		if model.YesAsk != 0 {
			t.Errorf("YesAsk = %d, want 0", model.YesAsk)
		}
		if model.LastPrice != 0 {
			t.Errorf("LastPrice = %d, want 0", model.LastPrice)
		}
	})

	t.Run("market with sub-penny prices", func(t *testing.T) {
		m := APIMarket{
			Ticker:           "SUBPENNY-MARKET",
			YesBidDollars:    "0.5255",
			YesAskDollars:    "0.5345",
			LastPriceDollars: "0.53001",
		}

		model := m.ToModel()

		if model.YesBid != 52550 {
			t.Errorf("YesBid = %d, want 52550", model.YesBid)
		}
		if model.YesAsk != 53450 {
			t.Errorf("YesAsk = %d, want 53450", model.YesAsk)
		}
		if model.LastPrice != 53001 {
			t.Errorf("LastPrice = %d, want 53001", model.LastPrice)
		}
	})
}

func TestAPIEventToModel(t *testing.T) {
	t.Run("full event conversion", func(t *testing.T) {
		e := APIEvent{
			EventTicker:   "TEST-EVENT",
			SeriesTicker:  "TEST-SERIES",
			Title:         "Test Event",
			Subtitle:      "Test subtitle",
			Category:      "Politics",
			Status:        "open",
			MarketTickers: []string{"MKT1", "MKT2"},
		}

		model := e.ToModel()

		if model.EventTicker != "TEST-EVENT" {
			t.Errorf("EventTicker = %q, want %q", model.EventTicker, "TEST-EVENT")
		}
		if model.SeriesTicker != "TEST-SERIES" {
			t.Errorf("SeriesTicker = %q, want %q", model.SeriesTicker, "TEST-SERIES")
		}
		if model.Title != "Test Event" {
			t.Errorf("Title = %q, want %q", model.Title, "Test Event")
		}
		if model.SubTitle != "Test subtitle" {
			t.Errorf("SubTitle = %q, want %q", model.SubTitle, "Test subtitle")
		}
		if model.Category != "Politics" {
			t.Errorf("Category = %q, want %q", model.Category, "Politics")
		}
		if model.UpdatedAt == 0 {
			t.Error("UpdatedAt should not be zero")
		}
	})

	t.Run("minimal event", func(t *testing.T) {
		e := APIEvent{
			EventTicker: "MINIMAL-EVENT",
		}

		model := e.ToModel()

		if model.EventTicker != "MINIMAL-EVENT" {
			t.Errorf("EventTicker = %q, want %q", model.EventTicker, "MINIMAL-EVENT")
		}
	})
}

func TestAPISeriesToModel(t *testing.T) {
	t.Run("full series conversion", func(t *testing.T) {
		s := APISeries{
			Ticker:            "TEST-SERIES",
			Title:             "Test Series",
			Category:          "Politics",
			Frequency:         "daily",
			Tags:              []string{"election", "2024"},
			SettlementSources: []string{"AP", "Reuters"},
		}

		model := s.ToModel()

		if model.Ticker != "TEST-SERIES" {
			t.Errorf("Ticker = %q, want %q", model.Ticker, "TEST-SERIES")
		}
		if model.Title != "Test Series" {
			t.Errorf("Title = %q, want %q", model.Title, "Test Series")
		}
		if model.Category != "Politics" {
			t.Errorf("Category = %q, want %q", model.Category, "Politics")
		}
		if model.Frequency != "daily" {
			t.Errorf("Frequency = %q, want %q", model.Frequency, "daily")
		}
		if len(model.Tags) != 2 {
			t.Errorf("len(Tags) = %d, want 2", len(model.Tags))
		}
		if model.Tags["election"] != "true" {
			t.Errorf("Tags[\"election\"] = %q, want %q", model.Tags["election"], "true")
		}
		if model.Tags["2024"] != "true" {
			t.Errorf("Tags[\"2024\"] = %q, want %q", model.Tags["2024"], "true")
		}
		if len(model.SettlementSources) != 2 {
			t.Errorf("len(SettlementSources) = %d, want 2", len(model.SettlementSources))
		}
		if model.UpdatedAt == 0 {
			t.Error("UpdatedAt should not be zero")
		}
	})

	t.Run("series with no tags", func(t *testing.T) {
		s := APISeries{
			Ticker: "NO-TAGS",
			Tags:   nil,
		}

		model := s.ToModel()

		if model.Tags == nil {
			t.Error("Tags should be initialized as empty map")
		}
		if len(model.Tags) != 0 {
			t.Errorf("len(Tags) = %d, want 0", len(model.Tags))
		}
	})

	t.Run("series with empty tags slice", func(t *testing.T) {
		s := APISeries{
			Ticker: "EMPTY-TAGS",
			Tags:   []string{},
		}

		model := s.ToModel()

		if len(model.Tags) != 0 {
			t.Errorf("len(Tags) = %d, want 0", len(model.Tags))
		}
	})
}

func TestOrderbookResponseToOrderbookSnapshot(t *testing.T) {
	t.Run("full orderbook", func(t *testing.T) {
		ob := &OrderbookResponse{
			Orderbook: APIOrderbook{
				Yes: [][]int{{52, 100}, {51, 200}, {50, 300}},
				No:  [][]int{{48, 150}, {47, 250}},
			},
		}

		snapshot := ob.ToOrderbookSnapshot("TEST-MARKET", "rest")

		if snapshot.Ticker != "TEST-MARKET" {
			t.Errorf("Ticker = %q, want %q", snapshot.Ticker, "TEST-MARKET")
		}
		if snapshot.Source != "rest" {
			t.Errorf("Source = %q, want %q", snapshot.Source, "rest")
		}
		if len(snapshot.YesBids) != 3 {
			t.Errorf("len(YesBids) = %d, want 3", len(snapshot.YesBids))
		}
		if len(snapshot.NoBids) != 2 {
			t.Errorf("len(NoBids) = %d, want 2", len(snapshot.NoBids))
		}

		// Check price conversion (cents to internal)
		if snapshot.YesBids[0].Price != 52000 {
			t.Errorf("YesBids[0].Price = %d, want 52000", snapshot.YesBids[0].Price)
		}
		if snapshot.YesBids[0].Size != 100 {
			t.Errorf("YesBids[0].Size = %d, want 100", snapshot.YesBids[0].Size)
		}

		// Best prices
		if snapshot.BestYesBid != 52000 {
			t.Errorf("BestYesBid = %d, want 52000", snapshot.BestYesBid)
		}
		// Best YES ask = 100000 - best NO bid (48 cents)
		if snapshot.BestYesAsk != 52000 { // 100000 - 48000
			t.Errorf("BestYesAsk = %d, want 52000", snapshot.BestYesAsk)
		}

		// Spread
		if snapshot.Spread != 0 { // 52000 - 52000
			t.Errorf("Spread = %d, want 0", snapshot.Spread)
		}

		// Timestamps
		if snapshot.SnapshotTS == 0 {
			t.Error("SnapshotTS should not be zero")
		}
		if snapshot.ExchangeTS != 0 {
			t.Errorf("ExchangeTS = %d, want 0 (not provided by REST)", snapshot.ExchangeTS)
		}
	})

	t.Run("empty orderbook", func(t *testing.T) {
		ob := &OrderbookResponse{
			Orderbook: APIOrderbook{
				Yes: [][]int{},
				No:  [][]int{},
			},
		}

		snapshot := ob.ToOrderbookSnapshot("EMPTY-MARKET", "ws")

		if len(snapshot.YesBids) != 0 {
			t.Errorf("len(YesBids) = %d, want 0", len(snapshot.YesBids))
		}
		if len(snapshot.NoBids) != 0 {
			t.Errorf("len(NoBids) = %d, want 0", len(snapshot.NoBids))
		}
		if snapshot.BestYesBid != 0 {
			t.Errorf("BestYesBid = %d, want 0", snapshot.BestYesBid)
		}
		if snapshot.BestYesAsk != 0 {
			t.Errorf("BestYesAsk = %d, want 0", snapshot.BestYesAsk)
		}
		if snapshot.Spread != 0 {
			t.Errorf("Spread = %d, want 0", snapshot.Spread)
		}
	})

	t.Run("only yes side", func(t *testing.T) {
		ob := &OrderbookResponse{
			Orderbook: APIOrderbook{
				Yes: [][]int{{52, 100}},
				No:  [][]int{},
			},
		}

		snapshot := ob.ToOrderbookSnapshot("YES-ONLY", "rest")

		if snapshot.BestYesBid != 52000 {
			t.Errorf("BestYesBid = %d, want 52000", snapshot.BestYesBid)
		}
		if snapshot.BestYesAsk != 0 {
			t.Errorf("BestYesAsk = %d, want 0", snapshot.BestYesAsk)
		}
		if snapshot.Spread != 0 {
			t.Errorf("Spread = %d, want 0 (no ask)", snapshot.Spread)
		}
	})

	t.Run("only no side", func(t *testing.T) {
		ob := &OrderbookResponse{
			Orderbook: APIOrderbook{
				Yes: [][]int{},
				No:  [][]int{{45, 100}},
			},
		}

		snapshot := ob.ToOrderbookSnapshot("NO-ONLY", "rest")

		if snapshot.BestYesBid != 0 {
			t.Errorf("BestYesBid = %d, want 0", snapshot.BestYesBid)
		}
		if snapshot.BestYesAsk != 55000 { // 100000 - 45000
			t.Errorf("BestYesAsk = %d, want 55000", snapshot.BestYesAsk)
		}
		if snapshot.Spread != 0 {
			t.Errorf("Spread = %d, want 0 (no bid)", snapshot.Spread)
		}
	})

	t.Run("malformed level (less than 2 elements)", func(t *testing.T) {
		ob := &OrderbookResponse{
			Orderbook: APIOrderbook{
				Yes: [][]int{{52}}, // Only price, no size
				No:  [][]int{{48, 100}},
			},
		}

		snapshot := ob.ToOrderbookSnapshot("MALFORMED", "rest")

		// Malformed levels should be skipped
		if len(snapshot.YesBids) != 0 {
			t.Errorf("len(YesBids) = %d, want 0 (malformed skipped)", len(snapshot.YesBids))
		}
		if len(snapshot.NoBids) != 1 {
			t.Errorf("len(NoBids) = %d, want 1", len(snapshot.NoBids))
		}
	})

	t.Run("spread calculation", func(t *testing.T) {
		ob := &OrderbookResponse{
			Orderbook: APIOrderbook{
				Yes: [][]int{{50, 100}}, // Best YES bid = 50000
				No:  [][]int{{45, 100}}, // Best YES ask = 100000 - 45000 = 55000
			},
		}

		snapshot := ob.ToOrderbookSnapshot("SPREAD-TEST", "rest")

		if snapshot.BestYesBid != 50000 {
			t.Errorf("BestYesBid = %d, want 50000", snapshot.BestYesBid)
		}
		if snapshot.BestYesAsk != 55000 {
			t.Errorf("BestYesAsk = %d, want 55000", snapshot.BestYesAsk)
		}
		if snapshot.Spread != 5000 { // 55000 - 50000
			t.Errorf("Spread = %d, want 5000", snapshot.Spread)
		}
	})
}

// Benchmark tests
func BenchmarkDollarsToInternal(b *testing.B) {
	for i := 0; i < b.N; i++ {
		DollarsToInternal("0.52505")
	}
}

func BenchmarkParseTimestamp(b *testing.B) {
	for i := 0; i < b.N; i++ {
		ParseTimestamp("2024-01-15T12:30:45Z")
	}
}

func BenchmarkAPIMarketToModel(b *testing.B) {
	m := APIMarket{
		Ticker:           "TEST-MARKET",
		EventTicker:      "TEST-EVENT",
		Title:            "Test Market",
		Status:           "open",
		YesBidDollars:    "0.52",
		YesAskDollars:    "0.54",
		LastPriceDollars: "0.53",
		Volume:           1000,
		OpenTime:         "2024-01-01T00:00:00Z",
		CloseTime:        "2024-12-31T23:59:59Z",
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		m.ToModel()
	}
}
