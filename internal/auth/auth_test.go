package auth

import (
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"encoding/pem"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestCredentials_SignRequest(t *testing.T) {
	// Generate a test key
	privateKey, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatalf("failed to generate test key: %v", err)
	}

	creds := &Credentials{
		KeyID:      "test-key-id",
		PrivateKey: privateKey,
	}

	headers, err := creds.SignRequest("GET", "/trade-api/ws/v2")
	if err != nil {
		t.Fatalf("SignRequest failed: %v", err)
	}

	// Verify all required headers are present
	if headers["KALSHI-ACCESS-KEY"] != "test-key-id" {
		t.Errorf("KALSHI-ACCESS-KEY = %q, want %q", headers["KALSHI-ACCESS-KEY"], "test-key-id")
	}

	if headers["KALSHI-ACCESS-TIMESTAMP"] == "" {
		t.Error("KALSHI-ACCESS-TIMESTAMP is empty")
	}

	if headers["KALSHI-ACCESS-SIGNATURE"] == "" {
		t.Error("KALSHI-ACCESS-SIGNATURE is empty")
	}

	// Signature should be base64 encoded
	if !isValidBase64(headers["KALSHI-ACCESS-SIGNATURE"]) {
		t.Errorf("KALSHI-ACCESS-SIGNATURE is not valid base64: %q", headers["KALSHI-ACCESS-SIGNATURE"])
	}
}

func TestCredentials_SignWebSocket(t *testing.T) {
	privateKey, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatalf("failed to generate test key: %v", err)
	}

	creds := &Credentials{
		KeyID:      "ws-key",
		PrivateKey: privateKey,
	}

	headers, err := creds.SignWebSocket()
	if err != nil {
		t.Fatalf("SignWebSocket failed: %v", err)
	}

	if headers["KALSHI-ACCESS-KEY"] != "ws-key" {
		t.Errorf("KALSHI-ACCESS-KEY = %q, want %q", headers["KALSHI-ACCESS-KEY"], "ws-key")
	}

	if headers["KALSHI-ACCESS-TIMESTAMP"] == "" {
		t.Error("KALSHI-ACCESS-TIMESTAMP is empty")
	}

	if headers["KALSHI-ACCESS-SIGNATURE"] == "" {
		t.Error("KALSHI-ACCESS-SIGNATURE is empty")
	}
}

func TestLoadPrivateKey_PKCS8(t *testing.T) {
	// Generate a test key
	privateKey, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatalf("failed to generate test key: %v", err)
	}

	// Encode as PKCS#8
	pkcs8Bytes, err := x509.MarshalPKCS8PrivateKey(privateKey)
	if err != nil {
		t.Fatalf("failed to marshal PKCS#8: %v", err)
	}

	pemBlock := &pem.Block{
		Type:  "PRIVATE KEY",
		Bytes: pkcs8Bytes,
	}

	// Write to temp file
	tmpFile := filepath.Join(t.TempDir(), "test-key.pem")
	if err := os.WriteFile(tmpFile, pem.EncodeToMemory(pemBlock), 0600); err != nil {
		t.Fatalf("failed to write temp file: %v", err)
	}

	// Load and verify
	loadedKey, err := LoadPrivateKey(tmpFile)
	if err != nil {
		t.Fatalf("LoadPrivateKey failed: %v", err)
	}

	if loadedKey.N.Cmp(privateKey.N) != 0 {
		t.Error("loaded key does not match original")
	}
}

func TestLoadPrivateKey_PKCS1(t *testing.T) {
	// Generate a test key
	privateKey, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatalf("failed to generate test key: %v", err)
	}

	// Encode as PKCS#1
	pkcs1Bytes := x509.MarshalPKCS1PrivateKey(privateKey)

	pemBlock := &pem.Block{
		Type:  "RSA PRIVATE KEY",
		Bytes: pkcs1Bytes,
	}

	// Write to temp file
	tmpFile := filepath.Join(t.TempDir(), "test-key.pem")
	if err := os.WriteFile(tmpFile, pem.EncodeToMemory(pemBlock), 0600); err != nil {
		t.Fatalf("failed to write temp file: %v", err)
	}

	// Load and verify
	loadedKey, err := LoadPrivateKey(tmpFile)
	if err != nil {
		t.Fatalf("LoadPrivateKey failed: %v", err)
	}

	if loadedKey.N.Cmp(privateKey.N) != 0 {
		t.Error("loaded key does not match original")
	}
}

func TestLoadPrivateKey_FileNotFound(t *testing.T) {
	_, err := LoadPrivateKey("/nonexistent/path/to/key.pem")
	if err == nil {
		t.Error("expected error for nonexistent file")
	}
}

func TestLoadPrivateKey_InvalidPEM(t *testing.T) {
	tmpFile := filepath.Join(t.TempDir(), "invalid.pem")
	if err := os.WriteFile(tmpFile, []byte("not a pem file"), 0600); err != nil {
		t.Fatalf("failed to write temp file: %v", err)
	}

	_, err := LoadPrivateKey(tmpFile)
	if err == nil {
		t.Error("expected error for invalid PEM")
	}
}

func TestLoadCredentials(t *testing.T) {
	// Generate a test key
	privateKey, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatalf("failed to generate test key: %v", err)
	}

	// Write key to temp file
	pkcs8Bytes, _ := x509.MarshalPKCS8PrivateKey(privateKey)
	pemBlock := &pem.Block{Type: "PRIVATE KEY", Bytes: pkcs8Bytes}
	tmpFile := filepath.Join(t.TempDir(), "test-key.pem")
	if err := os.WriteFile(tmpFile, pem.EncodeToMemory(pemBlock), 0600); err != nil {
		t.Fatalf("failed to write temp file: %v", err)
	}

	creds, err := LoadCredentials("my-key-id", tmpFile)
	if err != nil {
		t.Fatalf("LoadCredentials failed: %v", err)
	}

	if creds.KeyID != "my-key-id" {
		t.Errorf("KeyID = %q, want %q", creds.KeyID, "my-key-id")
	}

	if creds.PrivateKey == nil {
		t.Error("PrivateKey is nil")
	}
}

func TestLoadCredentials_MissingKeyID(t *testing.T) {
	_, err := LoadCredentials("", "/some/path")
	if err == nil {
		t.Error("expected error for missing key ID")
	}
}

func TestLoadCredentials_MissingPath(t *testing.T) {
	_, err := LoadCredentials("key-id", "")
	if err == nil {
		t.Error("expected error for missing path")
	}
}

func isValidBase64(s string) bool {
	// Base64 encoded string should only contain valid characters
	for _, c := range s {
		if !strings.ContainsRune("ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz0123456789+/=", c) {
			return false
		}
	}
	return len(s) > 0
}
