package macro

import (
	"os"
	"strconv"
	"time"

	"github.com/shopspring/decimal"
)

type MacroConfig struct {
	PollInterval       time.Duration
	StaleThreshold     int
	FredAPIKey         string
	FredBaseURL        string
	FredReleaseIDs     map[string]int
	FOMCCalendarURL    string
	TimeTolerance      time.Duration
	EventWindows       map[string]EventWindow
	DefaultWindow      EventWindow
	RelevanceMapping   map[string][]InstrumentRelevance
	AccuracyWindowSize int
	PostgresConnStr    string
	BLSAPIKey          string
	BLSBaseURL         string
	BLSSeriesIDs       map[string]string
	ScheduledEvents    []ScheduledEvent
}

func DefaultConfig() *MacroConfig {
	return &MacroConfig{
		PollInterval:   5 * time.Minute,
		StaleThreshold: 3,
		FredBaseURL:    "https://api.stlouisfed.org/fred",
		FredReleaseIDs: map[string]int{
			"GDP": 53,
			"CPI": 10,
			"NFP": 50,
		},
		FOMCCalendarURL: "https://www.federalreserve.gov/monetarypolicy/fomccalendars.htm",
		TimeTolerance:   5 * time.Minute,
		EventWindows: map[string]EventWindow{
			"FOMC": {PreMinutes: 15, PostMinutes: 60},
			"NFP":  {PreMinutes: 5, PostMinutes: 20},
			"CPI":  {PreMinutes: 5, PostMinutes: 30},
		},
		DefaultWindow:      EventWindow{PreMinutes: 10, PostMinutes: 30},
		RelevanceMapping:   defaultRelevanceMapping(),
		AccuracyWindowSize: 50,
		BLSBaseURL:         "https://api.bls.gov/publicAPI/v2/timeseries/data/",
		BLSSeriesIDs: map[string]string{
			"CPI": "CUUR0000SA0",
			"NFP": "CES0000000001",
		},
		ScheduledEvents: []ScheduledEvent{},
	}
}

func defaultRelevanceMapping() map[string][]InstrumentRelevance {
	return map[string][]InstrumentRelevance{
		"FOMC": {{Instrument: "BTC-USDT", Weight: decimal.NewFromFloat(0.8)}},
		"NFP":  {{Instrument: "BTC-USDT", Weight: decimal.NewFromFloat(0.6)}},
		"CPI":  {{Instrument: "BTC-USDT", Weight: decimal.NewFromFloat(0.7)}},
		"GDP":  {{Instrument: "BTC-USDT", Weight: decimal.NewFromFloat(0.4)}},
	}
}

func LoadConfigFromEnv() *MacroConfig {
	cfg := DefaultConfig()

	if v := os.Getenv("FRED_API_KEY"); v != "" {
		cfg.FredAPIKey = v
	}
	if v := os.Getenv("MACRO_POLL_INTERVAL_SEC"); v != "" {
		if sec, err := strconv.Atoi(v); err == nil {
			cfg.PollInterval = time.Duration(sec) * time.Second
		}
	}
	if v := os.Getenv("MACRO_STALE_THRESHOLD"); v != "" {
		if t, err := strconv.Atoi(v); err == nil {
			cfg.StaleThreshold = t
		}
	}
	if v := os.Getenv("MACRO_DB_CONN"); v != "" {
		cfg.PostgresConnStr = v
	}
	if v := os.Getenv("MACRO_TIME_TOLERANCE_SEC"); v != "" {
		if sec, err := strconv.Atoi(v); err == nil {
			cfg.TimeTolerance = time.Duration(sec) * time.Second
		}
	}
	if v := os.Getenv("BLS_API_KEY"); v != "" {
		cfg.BLSAPIKey = v
	}

	return cfg
}

func (c *MacroConfig) GetWindow(eventName string) EventWindow {
	if w, ok := c.EventWindows[eventName]; ok {
		return w
	}
	return c.DefaultWindow
}
