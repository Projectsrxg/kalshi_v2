package api

import (
	"log/slog"
	"net/http"
	"time"
)

// Client provides access to the Kalshi REST API.
type Client struct {
	baseURL    string
	apiKey     string
	httpClient *http.Client
	logger     *slog.Logger

	maxRetries   int
	retryBackoff time.Duration
}

// ClientOption configures a Client.
type ClientOption func(*Client)

// NewClient creates a new REST API client.
func NewClient(baseURL, apiKey string, opts ...ClientOption) *Client {
	c := &Client{
		baseURL: baseURL,
		apiKey:  apiKey,
		httpClient: &http.Client{
			Timeout: 30 * time.Second,
		},
		logger:       slog.Default(),
		maxRetries:   3,
		retryBackoff: time.Second,
	}

	for _, opt := range opts {
		opt(c)
	}

	return c
}

// WithTimeout sets the HTTP client timeout.
func WithTimeout(d time.Duration) ClientOption {
	return func(c *Client) {
		c.httpClient.Timeout = d
	}
}

// WithRetries sets the retry configuration.
func WithRetries(max int, backoff time.Duration) ClientOption {
	return func(c *Client) {
		c.maxRetries = max
		c.retryBackoff = backoff
	}
}

// WithLogger sets the logger.
func WithLogger(logger *slog.Logger) ClientOption {
	return func(c *Client) {
		c.logger = logger
	}
}

// WithHTTPClient sets a custom HTTP client.
func WithHTTPClient(hc *http.Client) ClientOption {
	return func(c *Client) {
		c.httpClient = hc
	}
}
