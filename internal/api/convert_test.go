package api

import (
	"testing"
)

func TestDollarsToInternal(t *testing.T) {
	tests := []struct {
		input string
		want  int
	}{
		{"0.52", 52000},
		{"0.5250", 52500},
		{"0.52505", 52505},
		{"0.00", 0},
		{"1.00", 100000},
		{"0.99999", 99999},
		{"0.01", 1000},
		{"", 0},
		{"invalid", 0},
		{"  0.52  ", 52000},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got := DollarsToInternal(tt.input)
			if got != tt.want {
				t.Errorf("DollarsToInternal(%q) = %d, want %d", tt.input, got, tt.want)
			}
		})
	}
}

func TestCentsToInternal(t *testing.T) {
	tests := []struct {
		input int
		want  int
	}{
		{52, 52000},
		{0, 0},
		{100, 100000},
		{1, 1000},
	}

	for _, tt := range tests {
		got := CentsToInternal(tt.input)
		if got != tt.want {
			t.Errorf("CentsToInternal(%d) = %d, want %d", tt.input, got, tt.want)
		}
	}
}

func TestParseTimestamp(t *testing.T) {
	// Test empty and invalid
	if got := ParseTimestamp(""); got != 0 {
		t.Errorf("ParseTimestamp(\"\") = %d, want 0", got)
	}
	if got := ParseTimestamp("invalid"); got != 0 {
		t.Errorf("ParseTimestamp(\"invalid\") = %d, want 0", got)
	}

	// Test valid RFC3339
	got := ParseTimestamp("2024-01-15T12:30:45Z")
	if got == 0 {
		t.Error("ParseTimestamp(\"2024-01-15T12:30:45Z\") = 0, want non-zero")
	}
	// Should be 1705321845000000 (2024-01-15 12:30:45 UTC in microseconds)
	if got != 1705321845000000 {
		t.Errorf("ParseTimestamp(\"2024-01-15T12:30:45Z\") = %d, want 1705321845000000", got)
	}
}

func TestAPIMarketToModel(t *testing.T) {
	m := APIMarket{
		Ticker:           "TEST-MARKET",
		EventTicker:      "TEST-EVENT",
		Title:            "Test Market",
		Subtitle:         "Test subtitle",
		Status:           "open",
		MarketType:       "binary",
		YesBidDollars:    "0.52",
		YesAskDollars:    "0.54",
		LastPriceDollars: "0.53",
		Volume:           1000,
		Volume24h:        500,
		OpenInterest:     200,
	}

	model := m.ToModel()

	if model.Ticker != "TEST-MARKET" {
		t.Errorf("Ticker = %q, want %q", model.Ticker, "TEST-MARKET")
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
}
