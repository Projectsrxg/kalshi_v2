package api

import (
	"context"
	"fmt"
	"net/url"
	"strconv"
	"strings"
)

// GetMarkets fetches a page of markets.
func (c *Client) GetMarkets(ctx context.Context, opts GetMarketsOptions) (*MarketsResponse, error) {
	query := url.Values{}

	if opts.Limit > 0 {
		query.Set("limit", strconv.Itoa(opts.Limit))
	}
	if opts.Cursor != "" {
		query.Set("cursor", opts.Cursor)
	}
	if opts.EventTicker != "" {
		query.Set("event_ticker", opts.EventTicker)
	}
	if opts.SeriesTicker != "" {
		query.Set("series_ticker", opts.SeriesTicker)
	}
	if len(opts.Tickers) > 0 {
		query.Set("tickers", strings.Join(opts.Tickers, ","))
	}
	if opts.Status != "" {
		query.Set("status", opts.Status)
	}

	var resp MarketsResponse
	if err := c.get(ctx, "/markets", query, &resp); err != nil {
		return nil, fmt.Errorf("get markets: %w", err)
	}

	return &resp, nil
}

// GetAllMarkets fetches all markets by paginating through results.
func (c *Client) GetAllMarkets(ctx context.Context) ([]APIMarket, error) {
	return c.GetAllMarketsWithOptions(ctx, GetMarketsOptions{})
}

// GetAllMarketsWithOptions fetches all markets matching the given options.
func (c *Client) GetAllMarketsWithOptions(ctx context.Context, opts GetMarketsOptions) ([]APIMarket, error) {
	var allMarkets []APIMarket
	opts.Limit = 1000 // Max page size

	for {
		resp, err := c.GetMarkets(ctx, opts)
		if err != nil {
			return nil, err
		}

		allMarkets = append(allMarkets, resp.Markets...)

		if resp.Cursor == "" {
			break
		}
		opts.Cursor = resp.Cursor
	}

	return allMarkets, nil
}

// GetMarket fetches a single market by ticker.
func (c *Client) GetMarket(ctx context.Context, ticker string) (*APIMarket, error) {
	var resp SingleMarketResponse
	if err := c.get(ctx, "/markets/"+ticker, nil, &resp); err != nil {
		return nil, fmt.Errorf("get market %s: %w", ticker, err)
	}
	return &resp.Market, nil
}

// GetOrderbook fetches the orderbook for a market.
func (c *Client) GetOrderbook(ctx context.Context, ticker string, depth int) (*OrderbookResponse, error) {
	query := url.Values{}
	if depth > 0 {
		query.Set("depth", strconv.Itoa(depth))
	}

	var resp OrderbookResponse
	if err := c.get(ctx, "/markets/"+ticker+"/orderbook", query, &resp); err != nil {
		return nil, fmt.Errorf("get orderbook %s: %w", ticker, err)
	}

	return &resp, nil
}
