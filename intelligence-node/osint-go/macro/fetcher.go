package macro

import (
	"context"
	"fmt"
	"log/slog"
	"strings"
	"time"
)

type Fetcher interface {
	Fetch(ctx context.Context) ([]EconomicEvent, error)
	Source() string
}

type FallbackFetcher struct {
	Primary    Fetcher
	Secondary  Fetcher
	Tolerance  time.Duration
	logger     *slog.Logger
}

func NewFallbackFetcher(primary, secondary Fetcher, tolerance time.Duration, logger *slog.Logger) *FallbackFetcher {
	return &FallbackFetcher{
		Primary:   primary,
		Secondary: secondary,
		Tolerance: tolerance,
		logger:    logger,
	}
}

type FetchResult struct {
	Events []EconomicEvent
	Source string
}

func (f *FallbackFetcher) Fetch(ctx context.Context) ([]EconomicEvent, error) {
	primaryEvents, primaryErr := f.Primary.Fetch(ctx)
	if primaryErr == nil {
		f.logger.Info("primary source succeeded",
			"source", f.Primary.Source(),
			"event_count", len(primaryEvents))

		secondaryEvents, secondaryErr := f.Secondary.Fetch(ctx)
		if secondaryErr == nil {
			f.crossValidate(primaryEvents, secondaryEvents)
		} else {
			f.logger.Warn("secondary source fetch failed (cross-validation skipped)",
				"source", f.Secondary.Source(),
				"error", secondaryErr)
		}

		return primaryEvents, nil
	}

	f.logger.Warn("primary source failed, trying secondary",
		"source", f.Primary.Source(),
		"error", primaryErr)

	secondaryEvents, secondaryErr := f.Secondary.Fetch(ctx)
	if secondaryErr == nil {
		f.logger.Info("secondary source succeeded (fallback)",
			"source", f.Secondary.Source(),
			"event_count", len(secondaryEvents))
		return secondaryEvents, nil
	}

	f.logger.Error("both sources unreachable",
		"primary_error", primaryErr,
		"secondary_error", secondaryErr)
	return nil, fmt.Errorf("both sources unreachable: primary: %w; secondary: %w", primaryErr, secondaryErr)
}

type MultiFetcher struct {
	fetchers []Fetcher
	logger   *slog.Logger
}

func NewMultiFetcher(logger *slog.Logger, fetchers ...Fetcher) *MultiFetcher {
	return &MultiFetcher{fetchers: fetchers, logger: logger}
}

func (m *MultiFetcher) Fetch(ctx context.Context) ([]EconomicEvent, error) {
	var allEvents []EconomicEvent
	var lastErr error

	for _, f := range m.fetchers {
		events, err := f.Fetch(ctx)
		if err != nil {
			m.logger.Warn("multi-fetcher: sub-fetcher failed",
				"source", f.Source(), "error", err)
			lastErr = err
			continue
		}
		allEvents = append(allEvents, events...)
	}

	if len(allEvents) == 0 && lastErr != nil {
		return nil, fmt.Errorf("multi-fetcher: all sub-fetchers failed: %w", lastErr)
	}

	return allEvents, nil
}

func (m *MultiFetcher) Source() string {
	sources := make([]string, 0, len(m.fetchers))
	for _, f := range m.fetchers {
		sources = append(sources, f.Source())
	}
	return "multi:" + strings.Join(sources, "+")
}

func (f *FallbackFetcher) crossValidate(primary, secondary []EconomicEvent) {
	secondaryByKey := make(map[string]EconomicEvent)
	for _, e := range secondary {
		key := e.Name + "|" + e.ScheduledAtUTC.Format("2006-01-02")
		secondaryByKey[key] = e
	}

	for _, pe := range primary {
		key := pe.Name + "|" + pe.ScheduledAtUTC.Format("2006-01-02")
		se, ok := secondaryByKey[key]
		if !ok {
			continue
		}

		delta := pe.ScheduledAtUTC.Sub(se.ScheduledAtUTC)
		if delta < 0 {
			delta = -delta
		}

		if delta > f.Tolerance {
			f.logger.Warn("CalendarSourceDiscrepancy",
				"event_id", pe.EventID,
				"event_name", pe.Name,
				"primary_time", pe.ScheduledAtUTC.Format(time.RFC3339),
				"secondary_time", se.ScheduledAtUTC.Format(time.RFC3339),
				"delta_minutes", delta.Minutes(),
				"threshold_minutes", f.Tolerance.Minutes(),
				"primary_source", f.Primary.Source(),
				"secondary_source", f.Secondary.Source(),
			)
		}
	}
}
