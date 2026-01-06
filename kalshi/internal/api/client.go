package api

import (
	"crypto/rsa"
	"log/slog"
	"net/http"
	"time"
)

// Client provides access to the Kalshi REST API.
type Client struct {
	baseURL    string
	keyID      string          // API key ID for KALSHI-ACCESS-KEY header
	privateKey *rsa.PrivateKey // RSA private key for signing requests
	httpClient *http.Client
	logger     *slog.Logger

	maxRetries   int
	retryBackoff time.Duration
}

// ClientOption configures a Client.
type ClientOption func(*Client)

// NewClient creates a new REST API client.
// keyID is the API key ID, privateKey is the RSA private key for signing.
// Pass nil for privateKey to make unauthenticated requests (will fail for most endpoints).
func NewClient(baseURL string, keyID string, privateKey *rsa.PrivateKey, opts ...ClientOption) *Client {
	c := &Client{
		baseURL:    baseURL,
		keyID:      keyID,
		privateKey: privateKey,
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
