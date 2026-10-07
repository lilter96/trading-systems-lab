package config

import (
	"os"
	"testing"
)

func TestLoad_Defaults(t *testing.T) {
	os.Unsetenv("WD_HEARTBEAT_PORT")
	os.Unsetenv("WD_ALLOWED_MISSED_SECONDS")
	os.Unsetenv("WD_BALANCE_POLL_SECONDS")
	os.Unsetenv("WD_EQUITY_FLOOR_FRACTION")
	os.Unsetenv("WD_STARTING_CAPITAL")
	os.Unsetenv("OKX_API_KEY")

	cfg, err := Load()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if cfg.HeartbeatPort != 5555 {
		t.Errorf("expected 5555, got %d", cfg.HeartbeatPort)
	}
	if cfg.AllowedMissedSeconds != 5 {
		t.Errorf("expected 5, got %d", cfg.AllowedMissedSeconds)
	}
	if cfg.BalancePollSeconds != 2 {
		t.Errorf("expected 2, got %d", cfg.BalancePollSeconds)
	}
	if cfg.EquityFloorFraction != 0.80 {
		t.Errorf("expected 0.80, got %f", cfg.EquityFloorFraction)
	}
	if cfg.StartingCapital != 10000 {
		t.Errorf("expected 10000, got %f", cfg.StartingCapital)
	}
	if cfg.OkxIsDemo != true {
		t.Errorf("expected true, got %v", cfg.OkxIsDemo)
	}
}

func TestLoad_FromEnv(t *testing.T) {
	t.Setenv("WD_HEARTBEAT_PORT", "6000")
	t.Setenv("WD_ALLOWED_MISSED_SECONDS", "10")
	t.Setenv("WD_BALANCE_POLL_SECONDS", "3")
	t.Setenv("WD_EQUITY_FLOOR_FRACTION", "0.50")
	t.Setenv("WD_STARTING_CAPITAL", "25000")
	t.Setenv("OKX_DEMO", "false")

	cfg, err := Load()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if cfg.HeartbeatPort != 6000 {
		t.Errorf("expected 6000, got %d", cfg.HeartbeatPort)
	}
	if cfg.AllowedMissedSeconds != 10 {
		t.Errorf("expected 10, got %d", cfg.AllowedMissedSeconds)
	}
	if cfg.BalancePollSeconds != 3 {
		t.Errorf("expected 3, got %d", cfg.BalancePollSeconds)
	}
	if cfg.EquityFloorFraction != 0.50 {
		t.Errorf("expected 0.50, got %f", cfg.EquityFloorFraction)
	}
	if cfg.StartingCapital != 25000 {
		t.Errorf("expected 25000, got %f", cfg.StartingCapital)
	}
	if cfg.OkxIsDemo != false {
		t.Errorf("expected false, got %v", cfg.OkxIsDemo)
	}
}

func TestLoad_InvalidIntFallback(t *testing.T) {
	t.Setenv("WD_HEARTBEAT_PORT", "not-a-number")

	cfg, err := Load()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if cfg.HeartbeatPort != 5555 {
		t.Errorf("expected default 5555, got %d", cfg.HeartbeatPort)
	}
}

func TestLoad_InvalidFloatFallback(t *testing.T) {
	t.Setenv("WD_EQUITY_FLOOR_FRACTION", "NaN")

	cfg, err := Load()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if cfg.EquityFloorFraction != 0.80 {
		t.Errorf("expected default 0.80, got %f", cfg.EquityFloorFraction)
	}
}

func TestLoad_OkxCredentials(t *testing.T) {
	t.Setenv("OKX_API_KEY", "test-key")
	t.Setenv("OKX_SECRET_KEY", "test-secret")
	t.Setenv("OKX_PASSPHRASE", "test-pass")

	cfg, err := Load()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if cfg.OkxApiKey != "test-key" {
		t.Errorf("expected test-key, got %s", cfg.OkxApiKey)
	}
	if cfg.OkxApiSecret != "test-secret" {
		t.Errorf("expected test-secret, got %s", cfg.OkxApiSecret)
	}
	if cfg.OkxPassphrase != "test-pass" {
		t.Errorf("expected test-pass, got %s", cfg.OkxPassphrase)
	}
}
