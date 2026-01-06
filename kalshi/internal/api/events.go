package api

import (
	"context"
	"fmt"
	"net/url"
	"strconv"
)

// GetEvents fetches a page of events.
func (c *Client) GetEvents(ctx context.Context, opts GetEventsOptions) (*EventsResponse, error) {
	query := url.Values{}

	if opts.Limit > 0 {
		query.Set("limit", strconv.Itoa(opts.Limit))
	}
	if opts.Cursor != "" {
		query.Set("cursor", opts.Cursor)
	}
	if opts.SeriesTicker != "" {
		query.Set("series_ticker", opts.SeriesTicker)
	}
	if opts.Status != "" {
		query.Set("status", opts.Status)
	}

	var resp EventsResponse
	if err := c.get(ctx, "/events", query, &resp); err != nil {
		return nil, fmt.Errorf("get events: %w", err)
	}

	return &resp, nil
}

// GetAllEvents fetches all events by paginating through results.
// Uses DefaultPaginationTimeout (10m) if the context has no deadline.
func (c *Client) GetAllEvents(ctx context.Context) ([]APIEvent, error) {
	// Apply default timeout if context has no deadline.
	if _, hasDeadline := ctx.Deadline(); !hasDeadline {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, DefaultPaginationTimeout)
		defer cancel()
	}

	var allEvents []APIEvent
	opts := GetEventsOptions{Limit: 1000}

	for {
		resp, err := c.GetEvents(ctx, opts)
		if err != nil {
			return nil, err
		}

		allEvents = append(allEvents, resp.Events...)

		if resp.Cursor == "" {
			break
		}
		opts.Cursor = resp.Cursor
	}

	return allEvents, nil
}

// GetEvent fetches a single event by ticker.
func (c *Client) GetEvent(ctx context.Context, eventTicker string) (*APIEvent, error) {
	var resp SingleEventResponse
	if err := c.get(ctx, "/events/"+eventTicker, nil, &resp); err != nil {
		return nil, fmt.Errorf("get event %s: %w", eventTicker, err)
	}
	return &resp.Event, nil
}
