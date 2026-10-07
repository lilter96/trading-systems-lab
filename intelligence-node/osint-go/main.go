package main

import (
	"context"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"sync"
	"syscall"
	"time"

	"github.com/shopspring/decimal"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/credentials/insecure"
	"intelligence-node/osint-go/macro"
	"intelligence-node/osint-go/news"
	"trading-system/contracts/go/intelligencepb"
)

type instrumentState struct {
	mu             sync.RWMutex
	macroScore     decimal.Decimal
	volRegime      macro.VolatilityRegime
	sentimentScore float64
	sentimentVol   int
}

func (s *instrumentState) updateMacro(score decimal.Decimal, regime macro.VolatilityRegime) {
	s.mu.Lock()
	s.macroScore = score
	s.volRegime = regime
	s.mu.Unlock()
}

func (s *instrumentState) updateSentiment(score float64, vol int) {
	s.mu.Lock()
	s.sentimentScore = score
	s.sentimentVol = vol
	s.mu.Unlock()
}

func (s *instrumentState) osintScore(instrument string) *intelligencepb.OsintScore {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return &intelligencepb.OsintScore{
		Instrument:        instrument,
		MacroComponent:    s.macroScore.StringFixed(6),
		SentimentComponent: fmt.Sprintf("%.6f", s.sentimentScore),
		SentimentVolume:   fmt.Sprintf("%d", s.sentimentVol),
		VolatilityRegime:  intelligencepb.VolatilityRegime(s.volRegime),
		ComputedAtUnixUtc: time.Now().UnixMilli(),
	}
}

func main() {
	slog.SetDefault(slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelInfo})))

	slog.Info("macro intelligence stream starting",
		"version", "0.1.0",
		"phase", "6.2")

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	logger := slog.Default()

	macroCfg := macro.LoadConfigFromEnv()
	macroStream := setupMacroStream(macroCfg, logger)
	go macroStream.Start(ctx)

	newsCfg := news.LoadNewsConfigFromEnv()
	newsStream := setupNewsStream(newsCfg, logger)
	go newsStream.Start(ctx)

	sentimentScorer := news.NewSentimentScorer(60, nil)
	gate := news.NewValidationGate(0.5)
	lexicon := news.DefaultLexiconScorer()
	sentimentStream := news.NewSentimentStream(newsStream.Output(), sentimentScorer, gate, lexicon, newsCfg, logger)
	go sentimentStream.Start(ctx)

	instruments := &instrumentState{}
	go startGRPCClient(ctx, instruments)
	go startHTTPServer(lexicon)

	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)

	printTicker := time.NewTicker(30 * time.Second)
	defer printTicker.Stop()

	slog.Info("all streams started — waiting for data",
		"news_sources", len(newsCfg.Sources))

	done := false
	for !done {
		select {
		case snapshot := <-macroStream.Output():
			for inst, regime := range snapshot.Regimes {
				instruments.updateMacro(snapshot.Score, regime)
				slog.Info("macro update",
					"instrument", inst,
					"score", snapshot.Score.StringFixed(6),
					"regime", regime.String(),
					"events", len(snapshot.ActiveEvents))
			}

		case sentiment := <-sentimentStream.Output():
			instruments.updateSentiment(sentiment.Score, sentiment.Volume)
			slog.Info("sentiment update",
				"instrument", sentiment.Instrument,
				"score", fmt.Sprintf("%.4f", sentiment.Score),
				"volume", sentiment.Volume)

		case <-printTicker.C:
			state := macroStream.CurrentState()
			slog.Info("heartbeat",
				"macro_score", state.Score.StringFixed(6),
				"macro_stale", state.Stale,
				"active_events", len(state.ActiveEvents),
				"regimes", formatRegimes(state.Regimes))

		case sig := <-sigCh:
			slog.Info("signal received, shutting down", "signal", sig.String())
			done = true

		case <-ctx.Done():
			done = true
		}
	}

	cancel()
	slog.Info("stream stopped")
}

func setupMacroStream(cfg *macro.MacroConfig, logger *slog.Logger) *macro.MacroStream {
	if cfg.FredAPIKey == "" {
		slog.Warn("FRED_API_KEY not set — primary source will be unavailable")
	}

	fredFetcher := macro.NewFREDFetcher(cfg.FredAPIKey, cfg.FredBaseURL, cfg.FredReleaseIDs, logger)
	fomcFetcher := macro.NewFOMCFetcher(cfg.FOMCCalendarURL, logger)
	blsFetcher := macro.NewBLSFetcher(cfg.BLSAPIKey, cfg.BLSBaseURL, cfg.BLSSeriesIDs, logger)
	primaryFetcher := macro.NewMultiFetcher(logger, fredFetcher, fomcFetcher, blsFetcher)
	scheduleFetcher := macro.NewScheduleFetcher(cfg.ScheduledEvents)
	fallbackFetcher := macro.NewFallbackFetcher(primaryFetcher, scheduleFetcher, cfg.TimeTolerance, logger)
	scorer := macro.NewMacroScorer(cfg)
	validator := macro.NewMacroValidator(cfg, logger)

	var outcomeStore *macro.OutcomeStore
	if cfg.PostgresConnStr != "" {
		var err error
		outcomeStore, err = macro.NewOutcomeStore(cfg.PostgresConnStr)
		if err != nil {
			slog.Warn("failed to connect to outcome store", "error", err)
		} else {
			defer outcomeStore.Close()
		}
	} else {
		slog.Info("MACRO_DB_CONN not set — outcomes will not be persisted")
	}

	return macro.NewMacroStream(fallbackFetcher, scorer, validator, outcomeStore, cfg, logger)
}

func setupNewsStream(cfg *news.NewsConfig, logger *slog.Logger) *news.NewsStream {
	fetchers := make([]news.Fetcher, 0, len(cfg.Sources))
	for _, src := range cfg.Sources {
		fetchers = append(fetchers, news.NewRSSFetcher(src.Name, src.URL, src.Tier))
	}
	multiFetcher := news.NewMultiFetcher(logger, fetchers...)
	cleaner := news.NewCleaner(cfg.TickerAliases, 15*time.Minute)
	return news.NewNewsStream(multiFetcher, cleaner, cfg, logger)
}

func startGRPCClient(ctx context.Context, state *instrumentState) {
	target := os.Getenv("GRPC_TARGET")
	if target == "" {
		target = "localhost:50053"
	}

	var opts []grpc.DialOption
	if os.Getenv("GRPC_INSECURE") == "true" {
		opts = append(opts, grpc.WithTransportCredentials(insecure.NewCredentials()))
	} else {
		creds := credentials.NewTLS(&tls.Config{InsecureSkipVerify: true})
		opts = append(opts, grpc.WithTransportCredentials(creds))
	}

	conn, err := grpc.NewClient(target, opts...)
	if err != nil {
		slog.Warn("gRPC client failed, continuing without streaming", "error", err)
		return
	}
	defer conn.Close()

	client := intelligencepb.NewIntelligenceFeedClient(conn)
	slog.Info("gRPC client connected", "target", target)

	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			stream, err := client.StreamOsintScores(ctx)
			if err != nil {
				slog.Warn("gRPC stream open failed, retrying next cycle", "error", err)
				continue
			}

			score := state.osintScore("BTC-USDT")
			if err := stream.Send(score); err != nil {
				slog.Warn("gRPC send failed, retrying next cycle", "error", err)
				continue
			}

			if _, err := stream.CloseAndRecv(); err != nil {
				slog.Warn("gRPC close failed", "error", err)
			}

		case <-ctx.Done():
			return
		}
	}
}

func startHTTPServer(lexicon *news.LexiconScorer) {
	addr := os.Getenv("EVALUATE_ADDR")
	if addr == "" {
		return
	}

	mux := http.NewServeMux()
	mux.HandleFunc("/evaluate", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "method not allowed", 405)
			return
		}

		var req struct {
			Texts []string `json:"texts"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, fmt.Sprintf("bad request: %v", err), 400)
			return
		}

		scores, err := lexicon.Score(req.Texts)
		if err != nil {
			http.Error(w, fmt.Sprintf("scoring error: %v", err), 500)
			return
		}

		resp := struct {
			Scores []float64 `json:"scores"`
		}{Scores: scores}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(resp)
	})

	slog.Info("HTTP evaluate server starting", "addr", addr)
	if err := http.ListenAndServe(addr, mux); err != nil {
		slog.Warn("HTTP server stopped", "error", err)
	}
}

func formatRegimes(regimes map[string]macro.VolatilityRegime) map[string]string {
	result := make(map[string]string, len(regimes))
	for inst, regime := range regimes {
		result[inst] = regime.String()
	}
	return result
}
