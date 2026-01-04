package api

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"math/rand/v2"
	"net/http"
	"net/url"
	"time"
)

// APIError represents an error from the Kalshi API.
type APIError struct {
	StatusCode int
	Message    string
	Body       []byte
}

func (e *APIError) Error() string {
	return fmt.Sprintf("kalshi api error %d: %s", e.StatusCode, e.Message)
}

// IsRetryable returns true if the error should trigger a retry.
func (e *APIError) IsRetryable() bool {
	return e.StatusCode >= 500 || e.StatusCode == 429
}

// doRequest performs an HTTP request with the given method and path.
func (c *Client) doRequest(ctx context.Context, method, path string, query url.Values) ([]byte, error) {
	fullURL := c.baseURL + path
	if len(query) > 0 {
		fullURL += "?" + query.Encode()
	}

	req, err := http.NewRequestWithContext(ctx, method, fullURL, nil)
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}

	req.Header.Set("Accept", "application/json")
	if c.apiKey != "" {
		req.Header.Set("Authorization", "Bearer "+c.apiKey)
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("do request: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("read response: %w", err)
	}

	if resp.StatusCode >= 400 {
		return nil, &APIError{
			StatusCode: resp.StatusCode,
			Message:    http.StatusText(resp.StatusCode),
			Body:       body,
		}
	}

	return body, nil
}

// doWithRetry performs a request with exponential backoff retry.
func (c *Client) doWithRetry(ctx context.Context, method, path string, query url.Values) ([]byte, error) {
	var lastErr error
	backoff := c.retryBackoff

	for attempt := 0; attempt <= c.maxRetries; attempt++ {
		if attempt > 0 {
			// Add jitter: backoff * (0.5 to 1.5)
			jitter := backoff / 2 + time.Duration(rand.Int64N(int64(backoff)))
			c.logger.Debug("retrying request",
				"attempt", attempt,
				"backoff", jitter,
				"path", path,
			)

			select {
			case <-ctx.Done():
				return nil, ctx.Err()
			case <-time.After(jitter):
			}

			backoff *= 2
		}

		body, err := c.doRequest(ctx, method, path, query)
		if err == nil {
			return body, nil
		}

		lastErr = err

		// Check if error is retryable
		apiErr, ok := err.(*APIError)
		if !ok || !apiErr.IsRetryable() {
			return nil, err
		}
	}

	return nil, fmt.Errorf("max retries exceeded: %w", lastErr)
}

// get performs a GET request with retries.
func (c *Client) get(ctx context.Context, path string, query url.Values, result any) error {
	body, err := c.doWithRetry(ctx, http.MethodGet, path, query)
	if err != nil {
		return err
	}

	if err := json.Unmarshal(body, result); err != nil {
		return fmt.Errorf("unmarshal response: %w", err)
	}

	return nil
}
