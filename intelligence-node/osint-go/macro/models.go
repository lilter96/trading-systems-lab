package macro

import (
	"time"

	"github.com/shopspring/decimal"
)

type EventImpact int8

const (
	EventImpactLow    EventImpact = 0
	EventImpactMedium EventImpact = 1
	EventImpactHigh   EventImpact = 2
)

func (e EventImpact) String() string {
	switch e {
	case EventImpactLow:
		return "Low"
	case EventImpactMedium:
		return "Medium"
	case EventImpactHigh:
		return "High"
	default:
		return "Unknown"
	}
}

type VolatilityRegime int8

const (
	RegimeUnknown           VolatilityRegime = 0
	RegimeNormal            VolatilityRegime = 1
	RegimeElevatedPreEvent  VolatilityRegime = 2
	RegimeElevatedPostEvent VolatilityRegime = 3
)

func (r VolatilityRegime) String() string {
	switch r {
	case RegimeUnknown:
		return "Unknown"
	case RegimeNormal:
		return "Normal"
	case RegimeElevatedPreEvent:
		return "ElevatedPreEvent"
	case RegimeElevatedPostEvent:
		return "ElevatedPostEvent"
	default:
		return "Unknown"
	}
}

type EconomicEvent struct {
	EventID            string
	Name               string
	Country            string
	Impact             EventImpact
	ScheduledAtUTC     time.Time
	Forecast           *decimal.Decimal
	Previous           *decimal.Decimal
	Actual             *decimal.Decimal
	ActualReleasedAtUTC *time.Time
	Source             string
}

type EventWindow struct {
	PreMinutes  int
	PostMinutes int
}

type InstrumentRelevance struct {
	Instrument string
	Weight     decimal.Decimal
}

type MacroSnapshot struct {
	Score        decimal.Decimal
	Regimes      map[string]VolatilityRegime
	ActiveEvents []EconomicEvent
	Stale        bool
	Timestamp    time.Time
}

type EventOutcome struct {
	EventID           string
	EventName         string
	Instrument        string
	Source            string
	SurpriseMagnitude decimal.Decimal
	RealizedVolPre    decimal.Decimal
	RealizedVolPost   decimal.Decimal
	DirectionAccuracy *bool
	CreatedAt         time.Time
}

type StalenessState struct {
	Source      string
	Failures    int
	LastSuccess time.Time
	IsStale     bool
}
