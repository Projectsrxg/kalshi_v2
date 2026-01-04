package api

import (
	"context"
	"fmt"
)

// GetSeries fetches a series by ticker.
func (c *Client) GetSeries(ctx context.Context, seriesTicker string) (*APISeries, error) {
	var resp SeriesResponse
	if err := c.get(ctx, "/series/"+seriesTicker, nil, &resp); err != nil {
		return nil, fmt.Errorf("get series %s: %w", seriesTicker, err)
	}
	return &resp.Series, nil
}
