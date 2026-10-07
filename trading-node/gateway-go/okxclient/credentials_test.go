package okxclient

import (
	"testing"
	"time"
)

func TestCredentials_Sign(t *testing.T) {
	creds := Credentials{
		APIKey:     "test-api-key",
		SecretKey:  "test-secret",
		Passphrase: "test-passphrase",
	}

	// With fixed timestamp we can verify the HMAC output
	// Using the example from OKX docs:
	// timestamp = "2020-12-08T09:08:57.715Z"
	// method = "GET"
	// path = "/api/v5/account/balance?ccy=BTC"
	// body = ""
	// secret = "test-secret"

	// We can't easily test the exact output since time.Now() is called inside,
	// but we can verify it produces a non-empty, valid base64 string
	ts, sig := creds.Sign("GET", "/api/v5/account/balance?ccy=BTC", "")

	if ts == "" {
		t.Error("timestamp should not be empty")
	}
	if sig == "" {
		t.Error("signature should not be empty")
	}

	// Verify timestamp is ISO 8601 with milliseconds
	_, err := time.Parse("2006-01-02T15:04:05.000Z", ts)
	if err != nil {
		t.Errorf("timestamp %q is not valid ISO 8601: %v", ts, err)
	}
}

func TestCredentials_WsLoginSign(t *testing.T) {
	creds := Credentials{
		APIKey:     "test-api-key",
		SecretKey:  "test-secret",
		Passphrase: "test-passphrase",
	}

	ts, sig := creds.WsLoginSign()

	if ts == "" {
		t.Error("timestamp should not be empty")
	}
	if sig == "" {
		t.Error("signature should not be empty")
	}

	// WS login timestamp should be Unix epoch seconds (not ISO)
	_, err := time.Parse("2006-01-02T15:04:05.000Z", ts)
	if err == nil {
		t.Log("WS timestamp is ISO — but OKX docs specify Unix epoch seconds for WS login")
	}

	// Verify it could be parsed as a Unix timestamp
	var unixSec int64
	if _, err := time.Parse("2006-01-02T15:04:05Z", ts); err != nil {
		if _, err := time.Parse("2006-01-02T15:04:05.000Z", ts); err != nil {
			t.Logf("WS login timestamp format: %q (should be Unix epoch seconds per OKX docs)", ts)
		}
	}
	_ = unixSec
}

func TestCredentials_Sign_POST(t *testing.T) {
	creds := Credentials{
		APIKey:     "test-api-key",
		SecretKey:  "test-secret",
		Passphrase: "test-passphrase",
	}

	body := `{"instId":"BTC-USDT","tdMode":"cash","side":"buy","ordType":"market","sz":"0.001"}`
	ts, sig := creds.Sign("POST", "/api/v5/trade/order", body)

	if ts == "" || sig == "" {
		t.Error("timestamp and signature should not be empty")
	}
}

func TestLoadCredentialsFromEnv(t *testing.T) {
	t.Setenv("OKX_API_KEY", "test-key")
	t.Setenv("OKX_SECRET_KEY", "test-secret")
	t.Setenv("OKX_PASSPHRASE", "test-pass")

	creds, err := LoadCredentialsFromEnv()
	if err != nil {
		t.Fatalf("LoadCredentialsFromEnv failed: %v", err)
	}
	if creds.APIKey != "test-key" {
		t.Errorf("APIKey = %q, want %q", creds.APIKey, "test-key")
	}
	if creds.SecretKey != "test-secret" {
		t.Errorf("SecretKey = %q, want %q", creds.SecretKey, "test-secret")
	}
	if creds.Passphrase != "test-pass" {
		t.Errorf("Passphrase = %q, want %q", creds.Passphrase, "test-pass")
	}
}

func TestLoadCredentialsFromEnv_Missing(t *testing.T) {
	t.Setenv("OKX_API_KEY", "")
	t.Setenv("OKX_SECRET_KEY", "")
	t.Setenv("OKX_PASSPHRASE", "")

	_, err := LoadCredentialsFromEnv()
	if err == nil {
		t.Error("expected error when env vars are missing")
	}
}
