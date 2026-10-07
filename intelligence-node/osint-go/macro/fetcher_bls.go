package macro

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"github.com/shopspring/decimal"
)

type BLSFetcher struct {
	apiKey    string
	baseURL   string
	seriesIDs map[string]string
	client    *http.Client
	logger    *slog.Logger
}

func NewBLSFetcher(apiKey, baseURL string, seriesIDs map[string]string, logger *slog.Logger) *BLSFetcher {
	return &BLSFetcher{
		apiKey:    apiKey,
		baseURL:   baseURL,
		seriesIDs: seriesIDs,
		client: &http.Client{
			Timeout: 30 * time.Second,
		},
		logger: logger.With("fetcher", "bls"),
	}
}

func (f *BLSFetcher) Source() string { return "bls" }

func (f *BLSFetcher) SeriesID(name string) string {
	return f.seriesIDs[name]
}

type blsSeriesResponse struct {
	Status  string     `json:"status"`
	Message []string   `json:"message"`
	Results blsResults `json:"Results"`
}

type blsResults struct {
	Series []blsSeries `json:"series"`
}

type blsSeries struct {
	SeriesID string         `json:"seriesID"`
	Data     []blsDataPoint `json:"data"`
}

type blsDataPoint struct {
	Year       string `json:"year"`
	Period     string `json:"period"`
	PeriodName string `json:"periodName"`
	Value      string `json:"value"`
}

func (f *BLSFetcher) Fetch(ctx context.Context) ([]EconomicEvent, error) {
	if f.apiKey == "" {
		return nil, fmt.Errorf("BLS API key not configured")
	}

	ids := make([]string, 0, len(f.seriesIDs))
	for _, id := range f.seriesIDs {
		ids = append(ids, id)
	}

	idToName := make(map[string]string, len(f.seriesIDs))
	for name, id := range f.seriesIDs {
		idToName[id] = name
	}

	now := time.Now().UTC()
	startYear := now.Year() - 1

	payload := map[string]interface{}{
		"seriesid":        ids,
		"startyear":       fmt.Sprintf("%d", startYear),
		"endyear":         fmt.Sprintf("%d", now.Year()),
		"registrationkey": f.apiKey,
	}

	body, err := json.Marshal(payload)
	if err != nil {
		return nil, fmt.Errorf("bls marshal payload: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, f.baseURL, strings.NewReader(string(body)))
	if err != nil {
		return nil, fmt.Errorf("bls request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := f.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("bls fetch failed: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("bls read failed: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("bls non-200 status: %d", resp.StatusCode)
	}

	var blsResp blsSeriesResponse
	if err := json.Unmarshal(respBody, &blsResp); err != nil {
		return nil, fmt.Errorf("bls parse error: %w", err)
	}

	if blsResp.Status != "REQUEST_SUCCEEDED" {
		msg := strings.Join(blsResp.Message, "; ")
		return nil, fmt.Errorf("bls API error: %s", msg)
	}

	var events []EconomicEvent

	for _, series := range blsResp.Results.Series {
		eventName, ok := idToName[series.SeriesID]
		if !ok || len(series.Data) == 0 {
			continue
		}

		latest := series.Data[0]

		yearInt, err := parseYear(latest.Year)
		if err != nil {
			f.logger.Warn("bls parse year", "series", series.SeriesID, "year", latest.Year, "error", err)
			continue
		}

		monthInt, err := parsePeriodMonth(latest.Period)
		if err != nil {
			f.logger.Warn("bls parse period", "series", series.SeriesID, "period", latest.Period, "error", err)
			continue
		}

		actualVal, err := decimal.NewFromString(latest.Value)
		if err != nil {
			f.logger.Warn("bls parse value", "series", series.SeriesID, "value", latest.Value, "error", err)
			continue
		}

		eventID := fmt.Sprintf("bls-%s-%d-%02d", eventName, yearInt, monthInt)
		events = append(events, EconomicEvent{
			EventID:        eventID,
			Name:           eventName,
			Country:        "US",
			Impact:         EventImpactHigh,
			ScheduledAtUTC: time.Date(yearInt, time.Month(monthInt), 1, 0, 0, 0, 0, time.UTC),
			Forecast:       nil,
			Previous:       nil,
			Actual:         &actualVal,
			Source:         "bls",
		})
	}

	if len(events) == 0 {
		return nil, fmt.Errorf("bls: no events found")
	}

	return events, nil
}

func parseYear(s string) (int, error) {
	if len(s) != 4 {
		return 0, fmt.Errorf("invalid year: %q", s)
	}
	var y int
	_, err := fmt.Sscanf(s, "%d", &y)
	return y, err
}

func parsePeriodMonth(s string) (int, error) {
	if !strings.HasPrefix(s, "M") {
		return 0, fmt.Errorf("invalid period format: %q", s)
	}
	var m int
	_, err := fmt.Sscanf(s, "M%d", &m)
	if err != nil || m < 1 || m > 12 {
		return 0, fmt.Errorf("invalid month in period: %q", s)
	}
	return m, nil
}
