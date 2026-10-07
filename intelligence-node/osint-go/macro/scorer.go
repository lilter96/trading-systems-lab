package macro

import (
	"sync"
	"time"

	"github.com/shopspring/decimal"
)

type MacroScorer struct {
	mu           sync.RWMutex
	config       *MacroConfig
	events       []EconomicEvent
	regimeCache  map[string]VolatilityRegime
	eventScores  map[string]decimal.Decimal
}

func NewMacroScorer(config *MacroConfig) *MacroScorer {
	return &MacroScorer{
		config:      config,
		regimeCache: make(map[string]VolatilityRegime),
		eventScores: make(map[string]decimal.Decimal),
	}
}

func (s *MacroScorer) UpdateEvents(events []EconomicEvent) {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.events = events
	s.regimeCache = make(map[string]VolatilityRegime)
	s.eventScores = make(map[string]decimal.Decimal)

	for _, ev := range events {
		relevantInstruments := s.relevantInstrumentsLocked(ev.Name)
		for _, ri := range relevantInstruments {
			s.regimeCache[ri.Instrument] = s.computeRegimeLocked(ev, ri.Instrument)
		}
		if ev.Actual != nil && ev.Forecast != nil {
			surprise := computeSurpriseMagnitude(*ev.Actual, *ev.Forecast)
			s.eventScores[ev.EventID] = surprise
		}
	}
}

func (s *MacroScorer) ComputeRegime(instrument string) VolatilityRegime {
	s.mu.RLock()
	defer s.mu.RUnlock()

	if regime, ok := s.regimeCache[instrument]; ok {
		return regime
	}
	return RegimeNormal
}

func (s *MacroScorer) computeRegimeLocked(ev EconomicEvent, instrument string) VolatilityRegime {
	window := s.config.GetWindow(ev.Name)
	now := time.Now().UTC()
	preStart := ev.ScheduledAtUTC.Add(-time.Duration(window.PreMinutes) * time.Minute)
	postEnd := ev.ScheduledAtUTC.Add(time.Duration(window.PostMinutes) * time.Minute)

	isPreEvent := !now.Before(preStart) && now.Before(ev.ScheduledAtUTC)
	isDuringEvent := !now.Before(ev.ScheduledAtUTC) && ev.Actual == nil
	isPostEvent := ev.Actual != nil && ev.ActualReleasedAtUTC != nil &&
		!now.Before(*ev.ActualReleasedAtUTC) && now.Before(postEnd)

	switch {
	case isPreEvent || isDuringEvent:
		return RegimeElevatedPreEvent
	case isPostEvent:
		return RegimeElevatedPostEvent
	default:
		return RegimeNormal
	}
}

func (s *MacroScorer) ComputeMacroScore() decimal.Decimal {
	s.mu.RLock()
	defer s.mu.RUnlock()

	if len(s.events) == 0 {
		return decimal.Zero
	}

	total := decimal.Zero
	count := 0

	for _, ev := range s.events {
		relevance := s.relevanceWeightLocked(ev.Name)
		if relevance.IsZero() {
			continue
		}

		surprise := decimal.Zero
		if ev.Actual != nil && ev.Forecast != nil {
			surprise = computeSurpriseMagnitude(*ev.Actual, *ev.Forecast)
		}

		impactWeight := decimal.NewFromInt(int64(ev.Impact) + 1).Div(decimal.NewFromInt(3))

		eventScore := surprise.Mul(impactWeight).Mul(relevance)
		total = total.Add(eventScore)
		count++
	}

	if count == 0 {
		return decimal.Zero
	}

	return total.Div(decimal.NewFromInt(int64(count)))
}

func computeSurpriseMagnitude(actual, forecast decimal.Decimal) decimal.Decimal {
	diff := actual.Sub(forecast).Abs()
	if forecast.IsZero() {
		return diff
	}
	return diff.Div(forecast.Abs())
}

func (s *MacroScorer) relevantInstrumentsLocked(eventName string) []InstrumentRelevance {
	if s.config.RelevanceMapping == nil {
		return nil
	}
	upperName := stringsToUpper(eventName)
	for key, list := range s.config.RelevanceMapping {
		if stringsToUpper(key) == upperName {
			return list
		}
	}
	return nil
}

func (s *MacroScorer) relevanceWeightLocked(eventName string) decimal.Decimal {
	relevant := s.relevantInstrumentsLocked(eventName)
	if len(relevant) == 0 {
		return decimal.Zero
	}
	maxWeight := decimal.Zero
	for _, r := range relevant {
		if r.Weight.GreaterThan(maxWeight) {
			maxWeight = r.Weight
		}
	}
	return maxWeight
}

func (s *MacroScorer) ActiveEvents() []EconomicEvent {
	s.mu.RLock()
	defer s.mu.RUnlock()

	result := make([]EconomicEvent, len(s.events))
	copy(result, s.events)
	return result
}

func stringsToUpper(s string) string {
	b := make([]byte, len(s))
	for i := 0; i < len(s); i++ {
		c := s[i]
		if c >= 'a' && c <= 'z' {
			c = c - 'a' + 'A'
		}
		b[i] = c
	}
	return string(b)
}
