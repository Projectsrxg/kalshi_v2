package writer

import (
	"encoding/json"
	"math"
	"strconv"

	"github.com/rickgao/kalshi-data/internal/router"
)

// dollarsToInternal converts a dollar string (e.g., "0.52") to internal
// integer format (hundred-thousandths: 52000).
func dollarsToInternal(dollars string) int {
	if dollars == "" {
		return 0
	}
	f, err := strconv.ParseFloat(dollars, 64)
	if err != nil {
		return 0
	}
	// Round to avoid floating point errors (e.g., 0.52 * 100000 = 51999.999...)
	return int(math.Round(f * 100000))
}

// sideToBoolean converts "yes"/"no" string to boolean (TRUE = yes, FALSE = no).
func sideToBoolean(side string) bool {
	return side == "yes"
}

// priceLevelJSON represents a price level in JSONB format.
type priceLevelJSON struct {
	Price int `json:"price"`
	Size  int `json:"size"`
}

// priceLevelsToJSONB converts router.PriceLevel slice to JSONB bytes.
func priceLevelsToJSONB(levels []router.PriceLevel) []byte {
	result := make([]priceLevelJSON, len(levels))
	for i, level := range levels {
		result[i] = priceLevelJSON{
			Price: dollarsToInternal(level.Dollars),
			Size:  level.Quantity,
		}
	}
	data, _ := json.Marshal(result)
	return data
}

// deriveAsksFromBids converts bids to asks on the opposite side.
// YES bid at X = NO ask at (100000 - X)
func deriveAsksFromBids(bids []router.PriceLevel) []byte {
	asks := make([]priceLevelJSON, len(bids))
	for i, bid := range bids {
		bidPrice := dollarsToInternal(bid.Dollars)
		askPrice := 100000 - bidPrice
		asks[i] = priceLevelJSON{
			Price: askPrice,
			Size:  bid.Quantity,
		}
	}
	data, _ := json.Marshal(asks)
	return data
}

// extractBestPrice returns the best price from price levels (first level).
func extractBestPrice(levels []router.PriceLevel) int {
	if len(levels) == 0 {
		return 0
	}
	return dollarsToInternal(levels[0].Dollars)
}

// extractBestAskFromBids derives the best ask from opposite-side bids.
func extractBestAskFromBids(bids []router.PriceLevel) int {
	if len(bids) == 0 {
		return 0
	}
	// Best bid on opposite side = best ask
	return 100000 - dollarsToInternal(bids[0].Dollars)
}
