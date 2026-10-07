package macro

import (
	"context"
	"log/slog"
	"sync"
	"time"

	"github.com/shopspring/decimal"
)

type MacroStream struct {
	fetcher      *FallbackFetcher
	scorer       *MacroScorer
	validator    *MacroValidator
	outcomeStore *OutcomeStore
	config       *MacroConfig
	logger       *slog.Logger

	staleness   *StalenessTracker
	output      chan MacroSnapshot
	current     MacroSnapshot
	mu          sync.RWMutex
	started     bool
}

func NewMacroStream(
	fetcher *FallbackFetcher,
	scorer *MacroScorer,
	validator *MacroValidator,
	outcomeStore *OutcomeStore,
	config *MacroConfig,
	logger *slog.Logger,
) *MacroStream {
	return &MacroStream{
		fetcher:      fetcher,
		scorer:       scorer,
		validator:    validator,
		outcomeStore: outcomeStore,
		config:       config,
		logger:       logger.With("component", "macro_stream"),
		staleness:    NewStalenessTracker(config.StaleThreshold),
		output:       make(chan MacroSnapshot, 16),
	}
}

func (m *MacroStream) Start(ctx context.Context) {
	m.mu.Lock()
	if m.started {
		m.mu.Unlock()
		return
	}
	m.started = true
	m.mu.Unlock()

	m.logger.Info("macro stream starting",
		"poll_interval", m.config.PollInterval,
		"primary_source", m.fetcher.Primary.Source(),
		"secondary_source", m.fetcher.Secondary.Source())

	if err := m.pollOnce(ctx); err != nil {
		m.logger.Warn("initial poll failed", "error", err)
		m.emitSnapshot(RegimeUnknown, decimal.Zero)
	}

	ticker := time.NewTicker(m.config.PollInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			if err := m.pollOnce(ctx); err != nil {
				m.logger.Error("poll failed", "error", err)
				m.staleness.RecordFailure(m.fetcher.Primary.Source())
				m.staleness.RecordFailure(m.fetcher.Secondary.Source())

				if m.staleness.IsStale(m.fetcher.Primary.Source()) &&
					m.staleness.IsStale(m.fetcher.Secondary.Source()) {
					m.logger.Warn("both sources stale, emitting Unknown regime")
					m.emitSnapshot(RegimeUnknown, decimal.Zero)
				}
			}

		case <-ctx.Done():
			m.logger.Info("macro stream stopped")
			return
		}
	}
}

func (m *MacroStream) pollOnce(ctx context.Context) error {
	events, err := m.fetcher.Fetch(ctx)
	if err != nil {
		return err
	}

	m.staleness.RecordSuccess(m.fetcher.Primary.Source())
	m.staleness.RecordSuccess(m.fetcher.Secondary.Source())

	m.scorer.UpdateEvents(events)

	activeEvents := m.scorer.ActiveEvents()
	instruments := m.collectInstruments(activeEvents)

	var highestRegime VolatilityRegime = RegimeNormal
	regimes := make(map[string]VolatilityRegime, len(instruments))

	for _, inst := range instruments {
		reg := m.scorer.ComputeRegime(inst)
		regimes[inst] = reg
		if reg > highestRegime {
			highestRegime = reg
		}
	}

	macroScore := m.scorer.ComputeMacroScore()

	for _, ev := range activeEvents {
		if ev.Actual != nil && ev.ActualReleasedAtUTC != nil {
			for _, inst := range instruments {
				m.validator.RecordPostEventOutcome(ev, inst)
			}
		}
	}

	snapshot := MacroSnapshot{
		Score:        macroScore,
		Regimes:      regimes,
		ActiveEvents: activeEvents,
		Stale:        false,
		Timestamp:    time.Now().UTC(),
	}

	m.mu.Lock()
	m.current = snapshot
	m.mu.Unlock()

	select {
	case m.output <- snapshot:
	default:
	}

	m.logger.Info("macro poll complete",
		"events", len(activeEvents),
		"instruments", len(instruments),
		"highest_regime", highestRegime,
		"macro_score", macroScore.StringFixed(6))

	return nil
}

func (m *MacroStream) emitSnapshot(regime VolatilityRegime, score decimal.Decimal) {
	instruments := []string{"BTC-USDT"}
	regimes := make(map[string]VolatilityRegime, len(instruments))
	for _, inst := range instruments {
		regimes[inst] = regime
	}

	snapshot := MacroSnapshot{
		Score:   score,
		Regimes: regimes,
		Stale:   regime == RegimeUnknown,
		Timestamp: time.Now().UTC(),
	}

	m.mu.Lock()
	m.current = snapshot
	m.mu.Unlock()

	select {
	case m.output <- snapshot:
	default:
	}
}

func (m *MacroStream) collectInstruments(events []EconomicEvent) []string {
	seen := make(map[string]struct{})
	for _, ev := range events {
		upperName := stringsToUpper(ev.Name)
		if relevants, ok := m.config.RelevanceMapping[upperName]; ok {
			for _, r := range relevants {
				seen[r.Instrument] = struct{}{}
			}
		}
	}

	if len(seen) == 0 {
		return []string{"BTC-USDT"}
	}

	result := make([]string, 0, len(seen))
	for inst := range seen {
		result = append(result, inst)
	}
	return result
}

func (m *MacroStream) CurrentState() MacroSnapshot {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.current
}

func (m *MacroStream) Output() <-chan MacroSnapshot {
	return m.output
}

type StalenessTracker struct {
	threshold   int
	mu          sync.Mutex
	failures    map[string]int
	lastSuccess map[string]time.Time
}

func NewStalenessTracker(threshold int) *StalenessTracker {
	return &StalenessTracker{
		threshold:   threshold,
		failures:    make(map[string]int),
		lastSuccess: make(map[string]time.Time),
	}
}

func (st *StalenessTracker) RecordSuccess(source string) {
	st.mu.Lock()
	defer st.mu.Unlock()
	st.failures[source] = 0
	st.lastSuccess[source] = time.Now().UTC()
}

func (st *StalenessTracker) RecordFailure(source string) bool {
	st.mu.Lock()
	defer st.mu.Unlock()
	st.failures[source]++
	return st.failures[source] >= st.threshold
}

func (st *StalenessTracker) IsStale(source string) bool {
	st.mu.Lock()
	defer st.mu.Unlock()
	return st.failures[source] >= st.threshold
}

func (st *StalenessTracker) State(source string) StalenessState {
	st.mu.Lock()
	defer st.mu.Unlock()
	return StalenessState{
		Source:      source,
		Failures:    st.failures[source],
		LastSuccess: st.lastSuccess[source],
		IsStale:     st.failures[source] >= st.threshold,
	}
}
