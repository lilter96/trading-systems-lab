package news

import (
	"context"
	"log/slog"
	"sync"
	"time"

	"intelligence-node/osint-go/internal/stale"
)

type SentimentSnapshot struct {
	Instrument       string
	Score            float64
	Volume           int
	MacroScore       float64
	VolatilityRegime int32
	ComputedAt       time.Time
	Stale            bool
}

type SentimentStream struct {
	scorer     *SentimentScorer
	gate       *ValidationGate
	lexicon    *LexiconScorer
	config     *NewsConfig
	logger     *slog.Logger
	staleness  *stale.Tracker
	articleCh  <-chan CleanedArticle
	output     chan SentimentSnapshot
	current    SentimentSnapshot
	mu         sync.RWMutex
	started    bool

	liveScorer Scorer
	liveReady  bool
}

func NewSentimentStream(
	articleCh <-chan CleanedArticle,
	scorer *SentimentScorer,
	gate *ValidationGate,
	lexicon *LexiconScorer,
	config *NewsConfig,
	logger *slog.Logger,
) *SentimentStream {
	return &SentimentStream{
		scorer:    scorer,
		gate:      gate,
		lexicon:   lexicon,
		config:    config,
		logger:    logger.With("component", "sentiment_stream"),
		staleness: stale.NewTracker(config.StaleThreshold),
		articleCh: articleCh,
		output:    make(chan SentimentSnapshot, 16),
		liveReady: false,
	}
}

func (ss *SentimentStream) Start(ctx context.Context) {
	ss.mu.Lock()
	if ss.started {
		ss.mu.Unlock()
		return
	}
	ss.started = true
	ss.mu.Unlock()

	ready, rho, err := ss.gate.IsLiveReady(ss.lexicon)
	if err != nil {
		ss.logger.Warn("validation gate check failed, falling back to lexicon", "error", err)
	}
	ss.liveReady = ready
	ss.liveScorer = ss.lexicon
	ss.logger.Info("sentiment stream starting",
		"poll_interval", ss.config.PollInterval,
		"gate_passed", ready,
		"spearman_rho", rho)

	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()

	instruments := make(map[string]bool)

	for {
		select {
		case article, ok := <-ss.articleCh:
			if !ok {
				return
			}
			ss.staleness.RecordSuccess("news_pipeline")

			scores, err := ss.liveScorer.Score([]string{article.CleanedText})
			if err != nil {
				ss.logger.Warn("scoring failed", "source", article.Source.SourceID, "error", err)
				continue
			}
			score := scores[0]

			for _, inst := range article.TaggedInstruments {
				instruments[inst] = true
				ss.scorer.Ingest(inst, article.DedupClusterID, article.Source.Tier, score)
			}
			if len(article.TaggedInstruments) == 0 {
				instruments["BTC-USDT"] = true
				ss.scorer.Ingest("BTC-USDT", article.DedupClusterID, article.Source.Tier, score)
			}

		case <-ticker.C:
			ss.emitSnapshots(instruments)

		case <-ctx.Done():
			ss.logger.Info("sentiment stream stopped")
			return
		}
	}
}

func (ss *SentimentStream) emitSnapshots(instruments map[string]bool) {
	allStale := true
	snapshots := make([]SentimentSnapshot, 0, len(instruments))

	for inst := range instruments {
		score, volume := ss.scorer.Aggregate(inst)
		snapshot := SentimentSnapshot{
			Instrument: inst,
			Score:      score,
			Volume:     volume,
			ComputedAt: time.Now().UTC(),
		}

		snapshots = append(snapshots, snapshot)

		if !ss.staleness.IsStale("news_pipeline") {
			allStale = false
			ss.mu.Lock()
			ss.current = snapshot
			ss.mu.Unlock()

			select {
			case ss.output <- snapshot:
			default:
			}
		}
	}

	if allStale {
		ss.logger.Warn("all sentiment sources stale")
	}
}

func (ss *SentimentStream) Output() <-chan SentimentSnapshot {
	return ss.output
}

func (ss *SentimentStream) CurrentState() SentimentSnapshot {
	ss.mu.RLock()
	defer ss.mu.RUnlock()
	return ss.current
}
