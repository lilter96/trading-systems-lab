package macro

import (
	"log/slog"
	"sync"
	"time"

	"github.com/shopspring/decimal"
)

type MacroValidator struct {
	mu       sync.RWMutex
	config   *MacroConfig
	logger   *slog.Logger
	outcomes []EventOutcome
	accuracy map[string]decimal.Decimal
}

func NewMacroValidator(config *MacroConfig, logger *slog.Logger) *MacroValidator {
	return &MacroValidator{
		config:   config,
		logger:   logger,
		outcomes: make([]EventOutcome, 0, config.AccuracyWindowSize),
		accuracy: make(map[string]decimal.Decimal),
	}
}

func (v *MacroValidator) RecordPostEventOutcome(event EconomicEvent, instrument string) {
	if event.Actual == nil || event.Forecast == nil {
		return
	}

	surprise := computeSurpriseMagnitude(*event.Actual, *event.Forecast)

	outcome := EventOutcome{
		EventID:           event.EventID,
		EventName:         event.Name,
		Instrument:        instrument,
		Source:            event.Source,
		SurpriseMagnitude: surprise,
		CreatedAt:         time.Now().UTC(),
	}

	v.mu.Lock()
	v.outcomes = append(v.outcomes, outcome)
	if len(v.outcomes) > v.config.AccuracyWindowSize {
		v.outcomes = v.outcomes[len(v.outcomes)-v.config.AccuracyWindowSize:]
	}
	v.recomputeAccuracyLocked()
	v.mu.Unlock()

	v.logger.Info("macro event outcome recorded",
		"event_id", event.EventID,
		"event_name", event.Name,
		"instrument", instrument,
		"surprise_magnitude", surprise.StringFixed(6),
	)
}

func (v *MacroValidator) recomputeAccuracyLocked() {
	typeCounts := make(map[string]struct {
		correct int
		total   int
	})

	for _, o := range v.outcomes {
		key := o.EventName
		entry := typeCounts[key]
		entry.total++
		if o.DirectionAccuracy != nil && *o.DirectionAccuracy {
			entry.correct++
		}
		typeCounts[key] = entry
	}

	for key, counts := range typeCounts {
		if counts.total == 0 {
			v.accuracy[key] = decimal.Zero
			continue
		}
		v.accuracy[key] = decimal.NewFromInt(int64(counts.correct)).
			Div(decimal.NewFromInt(int64(counts.total)))
	}
}

func (v *MacroValidator) GetAccuracy(eventType string) decimal.Decimal {
	v.mu.RLock()
	defer v.mu.RUnlock()

	acc, ok := v.accuracy[eventType]
	if !ok {
		return decimal.NewFromFloat(0.5)
	}
	return acc
}
