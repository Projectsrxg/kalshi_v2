package main

import (
	"context"
	"fmt"
	"log"
	"time"

	"github.com/rickgao/kalshi-data/internal/api"
)

func main() {
	// Use demo API (no auth required for reads)
	client := api.NewClient(
		"https://demo-api.kalshi.co/trade-api/v2",
		"",  // No API key needed for public endpoints
		nil, // No private key needed for public endpoints
		api.WithTimeout(30*time.Second),
	)

	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	// Test 1: Exchange Status
	fmt.Println("=== Testing Exchange Status ===")
	status, err := client.GetExchangeStatus(ctx)
	if err != nil {
		log.Fatalf("GetExchangeStatus failed: %v", err)
	}
	fmt.Printf("Exchange Active: %v\n", status.ExchangeActive)
	fmt.Printf("Trading Active: %v\n", status.TradingActive)

	// Test 2: Get Markets (first page)
	fmt.Println("\n=== Testing GetMarkets ===")
	markets, err := client.GetMarkets(ctx, api.GetMarketsOptions{Limit: 5})
	if err != nil {
		log.Fatalf("GetMarkets failed: %v", err)
	}
	fmt.Printf("Fetched %d markets (cursor: %q)\n", len(markets.Markets), markets.Cursor)
	for i, m := range markets.Markets {
		fmt.Printf("  %d. %s - %s (status: %s)\n", i+1, m.Ticker, m.Title, m.Status)
	}

	// Test 3: Get single market
	if len(markets.Markets) > 0 {
		ticker := markets.Markets[0].Ticker
		fmt.Printf("\n=== Testing GetMarket (%s) ===\n", ticker)
		market, err := client.GetMarket(ctx, ticker)
		if err != nil {
			log.Fatalf("GetMarket failed: %v", err)
		}
		fmt.Printf("Ticker: %s\n", market.Ticker)
		fmt.Printf("Title: %s\n", market.Title)
		fmt.Printf("Status: %s\n", market.Status)
		fmt.Printf("YesBid: %s (internal: %d)\n", market.YesBidDollars, api.DollarsToInternal(market.YesBidDollars))
		fmt.Printf("YesAsk: %s (internal: %d)\n", market.YesAskDollars, api.DollarsToInternal(market.YesAskDollars))

		// Test 4: Get Orderbook
		fmt.Printf("\n=== Testing GetOrderbook (%s) ===\n", ticker)
		ob, err := client.GetOrderbook(ctx, ticker, 5)
		if err != nil {
			log.Fatalf("GetOrderbook failed: %v", err)
		}
		fmt.Printf("YES levels: %d, NO levels: %d\n", len(ob.Orderbook.Yes), len(ob.Orderbook.No))
		if len(ob.Orderbook.Yes) > 0 {
			fmt.Println("Top YES levels:")
			for i, level := range ob.Orderbook.Yes {
				if i >= 3 {
					break
				}
				fmt.Printf("  Price: %d cents, Qty: %d\n", level[0], level[1])
			}
		}
	}

	// Test 5: Get Events
	fmt.Println("\n=== Testing GetEvents ===")
	events, err := client.GetEvents(ctx, api.GetEventsOptions{Limit: 3})
	if err != nil {
		log.Fatalf("GetEvents failed: %v", err)
	}
	fmt.Printf("Fetched %d events\n", len(events.Events))
	for i, e := range events.Events {
		fmt.Printf("  %d. %s - %s\n", i+1, e.EventTicker, e.Title)
	}

	fmt.Println("\n=== All API tests passed! ===")
}
