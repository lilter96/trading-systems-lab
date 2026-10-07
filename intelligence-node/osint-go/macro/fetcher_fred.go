package macro

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"time"
)

type FREDFetcher struct {
	apiKey      string
	baseURL     string
	releaseIDs  map[string]int
	releaseNames map[int]string
	client      *http.Client
	logger      *slog.Logger
}

func NewFREDFetcher(apiKey, baseURL string, releaseIDs map[string]int, logger *slog.Logger) *FREDFetcher {
	releaseNames := make(map[int]string, len(releaseIDs))
	for name, id := range releaseIDs {
		releaseNames[id] = name
	}

	return &FREDFetcher{
		apiKey:       apiKey,
		baseURL:      baseURL,
		releaseIDs:   releaseIDs,
		releaseNames: releaseNames,
		client: &http.Client{
			Timeout: 30 * time.Second,
		},
		logger: logger.With("fetcher", "fred"),
	}
}

func (f *FREDFetcher) Source() string { return "fred" }

func (f *FREDFetcher) Fetch(ctx context.Context) ([]EconomicEvent, error) {
	if f.apiKey == "" {
		return nil, fmt.Errorf("FRED API key not configured")
	}

	var allEvents []EconomicEvent

	for name, releaseID := range f.releaseIDs {
		url := fmt.Sprintf("%s/release/dates?release_id=%d&api_key=%s&file_type=json&include_release_dates_with_no_data=true&sort_order=desc&limit=100",
			f.baseURL, releaseID, f.apiKey)

		req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
		if err != nil {
			return nil, fmt.Errorf("fred request: %w", err)
		}

		resp, err := f.client.Do(req)
		if err != nil {
			f.logger.Warn("fred request failed", "release", name, "release_id", releaseID, "error", err)
			continue
		}

		body, err := io.ReadAll(resp.Body)
		resp.Body.Close()
		if err != nil {
			f.logger.Warn("fred read failed", "release", name, "error", err)
			continue
		}

		if resp.StatusCode != http.StatusOK {
			f.logger.Warn("fred non-200 status", "release", name, "status", resp.StatusCode)
			continue
		}

		events, err := parseFREDResponse(body, f.releaseNames)
		if err != nil {
			f.logger.Warn("fred parse failed", "release", name, "error", err)
			continue
		}

		allEvents = append(allEvents, events...)
	}

	if len(allEvents) == 0 {
		return nil, fmt.Errorf("fred: no events fetched from any release")
	}

	return allEvents, nil
}
