package news

import (
	"context"
	"log/slog"
	"sync"
	"time"

	"intelligence-node/osint-go/internal/stale"
)

type NewsStream struct {
	fetcher   *MultiFetcher
	cleaner   *Cleaner
	config    *NewsConfig
	logger    *slog.Logger
	staleness *stale.Tracker

	output  chan CleanedArticle
	current NewsSnapshot
	mu      sync.RWMutex
	started bool
}

func NewNewsStream(
	fetcher *MultiFetcher,
	cleaner *Cleaner,
	config *NewsConfig,
	logger *slog.Logger,
) *NewsStream {
	return &NewsStream{
		fetcher:   fetcher,
		cleaner:   cleaner,
		config:    config,
		logger:    logger.With("component", "news_stream"),
		staleness: stale.NewTracker(config.StaleThreshold),
		output:    make(chan CleanedArticle, 64),
	}
}

func (ns *NewsStream) Start(ctx context.Context) {
	ns.mu.Lock()
	if ns.started {
		ns.mu.Unlock()
		return
	}
	ns.started = true
	ns.mu.Unlock()

	ns.logger.Info("news stream starting",
		"poll_interval", ns.config.PollInterval,
		"sources", len(ns.config.Sources))

	if err := ns.pollOnce(ctx); err != nil {
		ns.logger.Warn("initial poll failed", "error", err)
	}

	ticker := time.NewTicker(ns.config.PollInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			if err := ns.pollOnce(ctx); err != nil {
				ns.logger.Error("news poll failed", "error", err)
			}

		case <-ctx.Done():
			ns.logger.Info("news stream stopped")
			return
		}
	}
}

func (ns *NewsStream) pollOnce(ctx context.Context) error {
	articles, err := ns.fetcher.Fetch(ctx)
	if err != nil {
		for _, src := range ns.fetcher.Sources() {
			ns.staleness.RecordFailure(src)
		}
		return err
	}

	for _, src := range ns.fetcher.Sources() {
		ns.staleness.RecordSuccess(src)
	}

	cleaned := ns.cleaner.Clean(articles)
	total, excluded := ns.cleaner.Stats()

	for _, ca := range cleaned {
		select {
		case ns.output <- ca:
		default:
		}
	}

	sourceHealth := make(map[string]SourceHealth, len(ns.fetcher.Sources()))
	for _, src := range ns.fetcher.Sources() {
		st := ns.staleness.State(src)
		sourceHealth[src] = SourceHealth{
			LastSuccess: st.LastSuccess,
			Failures:    st.Failures,
			IsStale:     st.IsStale,
		}
	}

	snapshot := NewsSnapshot{
		Articles: len(cleaned),
		Sources:  sourceHealth,
	}

	ns.mu.Lock()
	ns.current = snapshot
	ns.mu.Unlock()

	ns.logger.Info("news poll complete",
		"raw", len(articles),
		"cleaned", len(cleaned),
		"total_seen", total,
		"excluded_lang", excluded,
		"dedup_ratio", ratioSkipped(total, excluded+total-len(cleaned)))

	return nil
}

func (ns *NewsStream) Output() <-chan CleanedArticle {
	return ns.output
}

func (ns *NewsStream) CurrentState() NewsSnapshot {
	ns.mu.RLock()
	defer ns.mu.RUnlock()
	return ns.current
}

func ratioSkipped(total, skipped int) float64 {
	if total == 0 {
		return 0
	}
	return float64(skipped) / float64(total)
}
