package news

import (
	"context"
	"fmt"
	"log/slog"
	"strings"
)

type Fetcher interface {
	Fetch(ctx context.Context) ([]RawArticle, error)
	Source() string
	Tier() SourceTier
}

type MultiFetcher struct {
	fetchers []Fetcher
	logger   *slog.Logger
}

func NewMultiFetcher(logger *slog.Logger, fetchers ...Fetcher) *MultiFetcher {
	return &MultiFetcher{fetchers: fetchers, logger: logger}
}

func (m *MultiFetcher) Fetch(ctx context.Context) ([]RawArticle, error) {
	var all []RawArticle
	var lastErr error

	for _, f := range m.fetchers {
		articles, err := f.Fetch(ctx)
		if err != nil {
			if m.logger != nil {
				m.logger.Warn("news fetcher failed", "source", f.Source(), "tier", f.Tier(), "error", err)
			}
			lastErr = err
			continue
		}
		all = append(all, articles...)
	}

	if len(all) == 0 && lastErr != nil {
		return nil, fmt.Errorf("news: all fetchers failed: %w", lastErr)
	}

	return all, nil
}

func (m *MultiFetcher) Sources() []string {
	srcs := make([]string, 0, len(m.fetchers))
	for _, f := range m.fetchers {
		srcs = append(srcs, f.Source())
	}
	return srcs
}

func (m *MultiFetcher) Source() string {
	srcs := m.Sources()
	return "multi:" + strings.Join(srcs, "+")
}
