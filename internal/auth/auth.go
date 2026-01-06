// Package auth provides Kalshi API authentication using RSA-PSS signatures.
package auth

import (
	"crypto"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"crypto/x509"
	"encoding/base64"
	"encoding/pem"
	"fmt"
	"os"
	"time"
)

// Credentials holds the API key and private key for signing requests.
type Credentials struct {
	KeyID      string          // API key ID from Kalshi dashboard
	PrivateKey *rsa.PrivateKey // RSA private key for signing
}

// LoadCredentials loads credentials from key ID and private key file path.
func LoadCredentials(keyID, privateKeyPath string) (*Credentials, error) {
	if keyID == "" {
		return nil, fmt.Errorf("API key ID is required")
	}
	if privateKeyPath == "" {
		return nil, fmt.Errorf("private key path is required")
	}

	privateKey, err := LoadPrivateKey(privateKeyPath)
	if err != nil {
		return nil, fmt.Errorf("load private key: %w", err)
	}

	return &Credentials{
		KeyID:      keyID,
		PrivateKey: privateKey,
	}, nil
}

// LoadPrivateKey loads an RSA private key from a PEM file.
func LoadPrivateKey(path string) (*rsa.PrivateKey, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read key file: %w", err)
	}

	block, _ := pem.Decode(data)
	if block == nil {
		return nil, fmt.Errorf("failed to decode PEM block")
	}

	// Try PKCS#8 first (newer format)
	key, err := x509.ParsePKCS8PrivateKey(block.Bytes)
	if err == nil {
		rsaKey, ok := key.(*rsa.PrivateKey)
		if !ok {
			return nil, fmt.Errorf("key is not an RSA private key")
		}
		return rsaKey, nil
	}

	// Fall back to PKCS#1 (older format)
	rsaKey, err := x509.ParsePKCS1PrivateKey(block.Bytes)
	if err != nil {
		return nil, fmt.Errorf("parse private key: %w", err)
	}

	return rsaKey, nil
}

// SignRequest generates authentication headers for a Kalshi API request.
// For WebSocket connections, method should be "GET" and path should be "/trade-api/ws/v2".
func (c *Credentials) SignRequest(method, path string) (headers map[string]string, err error) {
	timestampMs := time.Now().UnixMilli()

	signature, err := c.generateSignature(timestampMs, method, path)
	if err != nil {
		return nil, err
	}

	return map[string]string{
		"KALSHI-ACCESS-KEY":       c.KeyID,
		"KALSHI-ACCESS-TIMESTAMP": fmt.Sprintf("%d", timestampMs),
		"KALSHI-ACCESS-SIGNATURE": signature,
	}, nil
}

// generateSignature creates an RSA-PSS signature for the given request.
// Message format: timestamp_ms + method + path
func (c *Credentials) generateSignature(timestampMs int64, method, path string) (string, error) {
	// Construct the message to sign
	message := fmt.Sprintf("%d%s%s", timestampMs, method, path)

	// Hash the message with SHA-256
	hashed := sha256.Sum256([]byte(message))

	// Sign with RSA-PSS
	signature, err := rsa.SignPSS(
		rand.Reader,
		c.PrivateKey,
		crypto.SHA256,
		hashed[:],
		&rsa.PSSOptions{SaltLength: rsa.PSSSaltLengthEqualsHash},
	)
	if err != nil {
		return "", fmt.Errorf("sign message: %w", err)
	}

	return base64.StdEncoding.EncodeToString(signature), nil
}

// WebSocketPath is the path used for WebSocket signature generation.
const WebSocketPath = "/trade-api/ws/v2"

// SignWebSocket generates authentication headers specifically for WebSocket connections.
func (c *Credentials) SignWebSocket() (headers map[string]string, err error) {
	return c.SignRequest("GET", WebSocketPath)
}
