package writer

import (
	"encoding/json"
	"testing"

	"github.com/rickgao/kalshi-data/internal/router"
)

func TestDollarsToInternal(t *testing.T) {
	tests := []struct {
		name     string
		dollars  string
		expected int
	}{
		{"standard penny", "0.52", 52000},
		{"subpenny 0.1 cent", "0.5250", 52500},
		{"subpenny 0.01 cent", "0.5255", 52550},
		{"near 100%", "0.99", 99000},
		{"tail pricing", "0.9999", 99990},
		{"near 0%", "0.01", 1000},
		{"exactly $1", "1.00", 100000},
		{"exactly $0", "0.00", 0},
		{"empty string", "", 0},
		{"invalid", "invalid", 0},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := dollarsToInternal(tt.dollars)
			if result != tt.expected {
				t.Errorf("dollarsToInternal(%q) = %d, want %d", tt.dollars, result, tt.expected)
			}
		})
	}
}

func TestSideToBoolean(t *testing.T) {
	tests := []struct {
		side     string
		expected bool
	}{
		{"yes", true},
		{"no", false},
		{"YES", false}, // Case-sensitive
		{"", false},
	}

	for _, tt := range tests {
		t.Run(tt.side, func(t *testing.T) {
			result := sideToBoolean(tt.side)
			if result != tt.expected {
				t.Errorf("sideToBoolean(%q) = %v, want %v", tt.side, result, tt.expected)
			}
		})
	}
}

func TestPriceLevelsToJSONB(t *testing.T) {
	levels := []router.PriceLevel{
		{Dollars: "0.52", Quantity: 100},
		{Dollars: "0.51", Quantity: 200},
	}

	result := priceLevelsToJSONB(levels)

	var parsed []priceLevelJSON
	if err := json.Unmarshal(result, &parsed); err != nil {
		t.Fatalf("failed to unmarshal result: %v", err)
	}

	if len(parsed) != 2 {
		t.Fatalf("expected 2 levels, got %d", len(parsed))
	}

	if parsed[0].Price != 52000 || parsed[0].Size != 100 {
		t.Errorf("level 0: got {%d, %d}, want {52000, 100}", parsed[0].Price, parsed[0].Size)
	}
	if parsed[1].Price != 51000 || parsed[1].Size != 200 {
		t.Errorf("level 1: got {%d, %d}, want {51000, 200}", parsed[1].Price, parsed[1].Size)
	}
}

func TestPriceLevelsToJSONB_Empty(t *testing.T) {
	result := priceLevelsToJSONB(nil)

	var parsed []priceLevelJSON
	if err := json.Unmarshal(result, &parsed); err != nil {
		t.Fatalf("failed to unmarshal result: %v", err)
	}

	if len(parsed) != 0 {
		t.Errorf("expected empty slice, got %d elements", len(parsed))
	}
}

func TestDeriveAsksFromBids(t *testing.T) {
	// YES bids at 52c → NO asks at 48c
	bids := []router.PriceLevel{
		{Dollars: "0.52", Quantity: 100},
		{Dollars: "0.51", Quantity: 200},
	}

	result := deriveAsksFromBids(bids)

	var parsed []priceLevelJSON
	if err := json.Unmarshal(result, &parsed); err != nil {
		t.Fatalf("failed to unmarshal result: %v", err)
	}

	if len(parsed) != 2 {
		t.Fatalf("expected 2 levels, got %d", len(parsed))
	}

	// 52000 bid → 48000 ask (100000 - 52000)
	if parsed[0].Price != 48000 || parsed[0].Size != 100 {
		t.Errorf("level 0: got {%d, %d}, want {48000, 100}", parsed[0].Price, parsed[0].Size)
	}
	// 51000 bid → 49000 ask
	if parsed[1].Price != 49000 || parsed[1].Size != 200 {
		t.Errorf("level 1: got {%d, %d}, want {49000, 200}", parsed[1].Price, parsed[1].Size)
	}
}

func TestExtractBestPrice(t *testing.T) {
	tests := []struct {
		name     string
		levels   []router.PriceLevel
		expected int
	}{
		{
			"with levels",
			[]router.PriceLevel{{Dollars: "0.52", Quantity: 100}, {Dollars: "0.51", Quantity: 200}},
			52000,
		},
		{
			"empty",
			nil,
			0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := extractBestPrice(tt.levels)
			if result != tt.expected {
				t.Errorf("extractBestPrice() = %d, want %d", result, tt.expected)
			}
		})
	}
}

func TestExtractBestAskFromBids(t *testing.T) {
	tests := []struct {
		name     string
		bids     []router.PriceLevel
		expected int
	}{
		{
			"with bids",
			[]router.PriceLevel{{Dollars: "0.52", Quantity: 100}},
			48000, // 100000 - 52000
		},
		{
			"empty",
			nil,
			0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := extractBestAskFromBids(tt.bids)
			if result != tt.expected {
				t.Errorf("extractBestAskFromBids() = %d, want %d", result, tt.expected)
			}
		})
	}
}
