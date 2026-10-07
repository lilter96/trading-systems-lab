package config

import (
	"math"
	"os"
	"strconv"
	"strings"
)

type Config struct {
	HeartbeatPort        int
	AllowedMissedSeconds int
	BalancePollSeconds   int
	EquityFloorFraction  float64
	StartingCapital      float64
	OkxApiKey            string
	OkxApiSecret         string
	OkxPassphrase        string
	OkxIsDemo            bool

	EmergencyServiceTargets []string
	GrpcCaCert              string
	GrpcWatchdogCert          string
	GrpcWatchdogKey           string
	TelegramBotToken        string
	TelegramChatId          string
}

func Load() (*Config, error) {
	cfg := &Config{
		HeartbeatPort:        envInt("WD_HEARTBEAT_PORT", 5555),
		AllowedMissedSeconds: envInt("WD_ALLOWED_MISSED_SECONDS", 5),
		BalancePollSeconds:   envInt("WD_BALANCE_POLL_SECONDS", 2),
		EquityFloorFraction:  envFloat("WD_EQUITY_FLOOR_FRACTION", 0.80),
		StartingCapital:      envFloat("WD_STARTING_CAPITAL", 10000),
		OkxApiKey:            envStr("OKX_API_KEY", ""),
		OkxApiSecret:         envStr("OKX_SECRET_KEY", ""),
		OkxPassphrase:        envStr("OKX_PASSPHRASE", ""),
		OkxIsDemo:            envBool("OKX_DEMO", true),
		EmergencyServiceTargets: envList("WD_EMERGENCY_TARGETS", nil),
		GrpcCaCert:             envStr("GRPC_CA_CERT", "deployment/certs/ca.crt"),
		GrpcWatchdogCert:         envStr("GRPC_WATCHDOG_CERT", "deployment/certs/watchdog.crt"),
		GrpcWatchdogKey:          envStr("GRPC_WATCHDOG_KEY", "deployment/certs/watchdog.key"),
		TelegramBotToken:       envStr("WD_TELEGRAM_BOT_TOKEN", ""),
		TelegramChatId:         envStr("WD_TELEGRAM_CHAT_ID", ""),
	}

	return cfg, nil
}

func envStr(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

func envInt(key string, fallback int) int {
	raw := os.Getenv(key)
	if raw == "" {
		return fallback
	}
	v, err := strconv.Atoi(raw)
	if err != nil {
		return fallback
	}
	return v
}

func envFloat(key string, fallback float64) float64 {
	raw := os.Getenv(key)
	if raw == "" {
		return fallback
	}
	v, err := strconv.ParseFloat(raw, 64)
	if err != nil {
		return fallback
	}
	if math.IsNaN(v) || math.IsInf(v, 0) {
		return fallback
	}
	return v
}

func envList(key string, fallback []string) []string {
	raw := os.Getenv(key)
	if raw == "" {
		return fallback
	}
	var result []string
	for _, item := range strings.Split(raw, ",") {
		trimmed := strings.TrimSpace(item)
		if trimmed != "" {
			result = append(result, trimmed)
		}
	}
	return result
}

func envBool(key string, fallback bool) bool {
	raw := os.Getenv(key)
	if raw == "" {
		return fallback
	}
	raw = strings.ToLower(raw)
	return raw == "true" || raw == "1" || raw == "yes"
}
