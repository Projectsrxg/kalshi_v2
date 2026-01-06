package api

import (
	"context"
	"fmt"
)

// GetExchangeStatus fetches the current exchange status.
func (c *Client) GetExchangeStatus(ctx context.Context) (*ExchangeStatusResponse, error) {
	var resp ExchangeStatusResponse
	if err := c.get(ctx, "/exchange/status", nil, &resp); err != nil {
		return nil, fmt.Errorf("get exchange status: %w", err)
	}
	return &resp, nil
}
