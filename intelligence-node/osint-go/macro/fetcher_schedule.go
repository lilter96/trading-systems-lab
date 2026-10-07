package macro

import (
	"context"
	"fmt"
	"time"
)

type ScheduleFetcher struct {
	events []EconomicEvent
}

func NewScheduleFetcher(entries []ScheduledEvent) *ScheduleFetcher {
	events := make([]EconomicEvent, 0, len(entries))
	for _, se := range entries {
		events = append(events, EconomicEvent{
			EventID:        fmt.Sprintf("schedule-%s-%s", se.Name, se.ScheduledAtUTC.Format("20060102")),
			Name:           se.Name,
			Country:        "US",
			Impact:         EventImpactHigh,
			ScheduledAtUTC: se.ScheduledAtUTC,
			Source:         "known_schedule",
		})
	}
	return &ScheduleFetcher{events: events}
}

func (f *ScheduleFetcher) Source() string { return "known_schedule" }

func (f *ScheduleFetcher) Fetch(_ context.Context) ([]EconomicEvent, error) {
	if len(f.events) == 0 {
		return nil, fmt.Errorf("known_schedule: no entries configured")
	}
	return f.events, nil
}

type ScheduledEvent struct {
	Name           string
	ScheduledAtUTC time.Time
}
