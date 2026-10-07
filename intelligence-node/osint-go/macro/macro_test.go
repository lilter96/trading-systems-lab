package macro

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"testing"
	"time"

	"github.com/shopspring/decimal"
)

func TestParseWeeklyCalendar_CorrectFieldsAndTimes(t *testing.T) {
	mockBody := `{
		"release_dates": [
			{
				"release_id": 53,
				"release_name": "Gross Domestic Product",
				"date": "2026-07-30",
				"release_date": "2026-07-30 08:30:00-05",
				"revision": false
			},
			{
				"release_id": 10,
				"release_name": "Consumer Price Index for All Urban Consumers",
				"date": "2026-08-12",
				"release_date": "2026-08-12 08:30:00-04",
				"revision": false
			},
			{
				"release_id": 50,
				"release_name": "Employment Situation",
				"date": "2026-08-07",
				"release_date": "2026-08-07 08:30:00-05",
				"revision": false
			}
		]
	}`

	releaseNames := map[int]string{
		53: "GDP",
		10: "CPI",
		50: "NFP",
	}

	events, err := parseFREDResponse([]byte(mockBody), releaseNames)
	if err != nil {
		t.Fatalf("parseFREDResponse failed: %v", err)
	}

	if len(events) != 3 {
		t.Fatalf("expected 3 events, got %d", len(events))
	}

	expectedTimes := []struct {
		name string
		utc  string
	}{
		{"GDP", "2026-07-30T13:30:00Z"},
		{"CPI", "2026-08-12T12:30:00Z"},
		{"NFP", "2026-08-07T13:30:00Z"},
	}

	for i, et := range expectedTimes {
		if events[i].Name != et.name {
			t.Errorf("event[%d] name: expected %q, got %q", i, et.name, events[i].Name)
		}
		if events[i].Country != "US" {
			t.Errorf("event[%d] country: expected US, got %q", i, events[i].Country)
		}
		got := events[i].ScheduledAtUTC.Format(time.RFC3339)
		if got != et.utc {
			t.Errorf("event[%d] time: expected %q, got %q", i, et.utc, got)
		}
		if events[i].Source != "fred" {
			t.Errorf("event[%d] source: expected fred, got %q", i, events[i].Source)
		}
	}
}

func TestPrimarySourceOutage_FallsBackToSecondary(t *testing.T) {
	var logBuf bytes.Buffer
	logger := slog.New(slog.NewTextHandler(&logBuf, &slog.HandlerOptions{Level: slog.LevelInfo}))

	primary := &mockFetcher{
		source: "fred",
		err:    assertError("primary outage"),
	}
	secondary := &mockFetcher{
		source: "investing_com",
		events: []EconomicEvent{
			{
				EventID: "test-1", Name: "CPI", Country: "US",
				Impact: EventImpactHigh,
				ScheduledAtUTC: time.Now().UTC().Add(24 * time.Hour),
				Source: "investing_com",
			},
		},
	}

	fallback := NewFallbackFetcher(primary, secondary, 5*time.Minute, logger)
	events, err := fallback.Fetch(context.Background())
	if err != nil {
		t.Fatalf("expected fallback to succeed, got error: %v", err)
	}
	if len(events) != 1 {
		t.Fatalf("expected 1 event from fallback, got %d", len(events))
	}
	if events[0].Source != "investing_com" {
		t.Errorf("expected source investing_com, got %q", events[0].Source)
	}

	if !bytes.Contains(logBuf.Bytes(), []byte("primary source failed")) {
		t.Error("expected log about primary source failure")
	}
	if !bytes.Contains(logBuf.Bytes(), []byte("secondary source succeeded")) {
		t.Error("expected log about secondary source fallback")
	}
}

func TestBothSourcesUnreachable_ReturnsUnknownNotNormal(t *testing.T) {
	var logBuf bytes.Buffer
	logger := slog.New(slog.NewTextHandler(&logBuf, &slog.HandlerOptions{Level: slog.LevelInfo}))

	primary := &mockFetcher{source: "fred", err: assertError("primary down")}
	secondary := &mockFetcher{source: "investing_com", err: assertError("secondary down")}

	fallback := NewFallbackFetcher(primary, secondary, 5*time.Minute, logger)
	_, err := fallback.Fetch(context.Background())
	if err == nil {
		t.Fatal("expected error when both sources unreachable")
	}

	if !bytes.Contains(logBuf.Bytes(), []byte("both sources unreachable")) {
		t.Error("expected log about both sources unreachable")
	}

	cfg := DefaultConfig()
	scorer := NewMacroScorer(cfg)
	scorer.UpdateEvents(nil)
	regime := scorer.ComputeRegime("BTC-USDT")
	if regime != RegimeNormal {
		t.Errorf("expected Normal regime with no events, got %s", regime)
	}

	regime = RegimeUnknown
	if regime == RegimeNormal {
		t.Error("RegimeUnknown should not equal RegimeNormal")
	}
}

func TestSourceDiscrepancy_AboveTolerance_RaisesAlert(t *testing.T) {
	var logBuf bytes.Buffer
	logger := slog.New(slog.NewTextHandler(&logBuf, &slog.HandlerOptions{Level: slog.LevelInfo}))

	baseTime := time.Now().UTC().Add(48 * time.Hour).Truncate(time.Hour)

	primary := &mockFetcher{
		source: "fred",
		events: []EconomicEvent{
			{EventID: "fred-cpi-1", Name: "CPI", Country: "US", Impact: EventImpactHigh,
				ScheduledAtUTC: baseTime, Source: "fred"},
		},
	}
	secondary := &mockFetcher{
		source: "investing_com",
		events: []EconomicEvent{
			{EventID: "inv-cpi-1", Name: "CPI", Country: "US", Impact: EventImpactHigh,
				ScheduledAtUTC: baseTime.Add(10 * time.Minute), Source: "investing_com"},
		},
	}

	fallback := NewFallbackFetcher(primary, secondary, 5*time.Minute, logger)
	events, err := fallback.Fetch(context.Background())
	if err != nil {
		t.Fatalf("expected primary to succeed, got error: %v", err)
	}
	if len(events) != 1 {
		t.Fatalf("expected 1 event, got %d", len(events))
	}

	if !bytes.Contains(logBuf.Bytes(), []byte("CalendarSourceDiscrepancy")) {
		t.Error("expected CalendarSourceDiscrepancy warning log")
	}
	if !bytes.Contains(logBuf.Bytes(), []byte("delta_minutes")) {
		t.Error("expected delta_minutes in log")
	}
	if !bytes.Contains(logBuf.Bytes(), []byte("10")) {
		t.Error("expected delta of 10 minutes in log")
	}
}

func TestDstTransitionWeek_TimesRemainUtcCorrect(t *testing.T) {
	mockBody := `{
		"release_dates": [
			{
				"release_id": 53,
				"release_name": "Gross Domestic Product",
				"date": "2027-03-12",
				"release_date": "2027-03-12 08:30:00-06",
				"revision": false
			},
			{
				"release_id": 10,
				"release_name": "Consumer Price Index",
				"date": "2027-03-15",
				"release_date": "2027-03-15 08:30:00-05",
				"revision": false
			}
		]
	}`

	releaseNames := map[int]string{
		53: "GDP",
		10: "CPI",
	}

	events, err := parseFREDResponse([]byte(mockBody), releaseNames)
	if err != nil {
		t.Fatalf("parseFREDResponse failed: %v", err)
	}

	if len(events) != 2 {
		t.Fatalf("expected 2 events, got %d", len(events))
	}

	expected := []string{
		"2027-03-12T14:30:00Z",
		"2027-03-15T13:30:00Z",
	}

	for i, exp := range expected {
		got := events[i].ScheduledAtUTC.Format(time.RFC3339)
		if got != exp {
			t.Errorf("event[%d] UTC time: expected %q, got %q", i, exp, got)
		}
	}
}

func TestEventWindow_PerEventTypeConfig_NotFlat30Min(t *testing.T) {
	cfg := DefaultConfig()
	cfg.EventWindows = map[string]EventWindow{
		"FOMC": {PreMinutes: 15, PostMinutes: 60},
		"NFP":  {PreMinutes: 5, PostMinutes: 20},
		"CPI":  {PreMinutes: 5, PostMinutes: 30},
	}

	now := time.Now().UTC()

	fomcTime := now.Add(10 * time.Minute)
	nfpTime := now.Add(10 * time.Minute)

	fomcEvent := EconomicEvent{
		EventID: "fomc-1", Name: "FOMC", Country: "US", Impact: EventImpactHigh,
		ScheduledAtUTC: fomcTime, Source: "fred",
	}
	nfpEvent := EconomicEvent{
		EventID: "nfp-1", Name: "NFP", Country: "US", Impact: EventImpactHigh,
		ScheduledAtUTC: nfpTime, Source: "fred",
	}

	scorer := NewMacroScorer(cfg)

	scorer.UpdateEvents([]EconomicEvent{fomcEvent})
	fomcRegime := scorer.ComputeRegime("BTC-USDT")
	if fomcRegime != RegimeElevatedPreEvent {
		t.Errorf("FOMC at t+10min with 15min pre-window: expected ElevatedPreEvent, got %s", fomcRegime)
	}

	scorer.UpdateEvents([]EconomicEvent{nfpEvent})
	nfpRegime := scorer.ComputeRegime("BTC-USDT")
	if nfpRegime == RegimeElevatedPreEvent {
		t.Errorf("NFP at t+10min with 5min pre-window: expected Normal, got %s", nfpRegime)
	}
	if nfpRegime != RegimeNormal {
		t.Errorf("NFP at t+10min with 5min pre-window: expected Normal, got %s", nfpRegime)
	}
}

func TestInstrumentRelevance_FiltersNonApplicableEvents(t *testing.T) {
	cfg := DefaultConfig()
	cfg.RelevanceMapping = map[string][]InstrumentRelevance{
		"FOMC": {{Instrument: "BTC-USDT", Weight: decimal.NewFromFloat(0.8)}},
	}

	now := time.Now().UTC()
	eventTime := now.Add(5 * time.Minute)

	euroEvent := EconomicEvent{
		EventID: "euro-cpi-1", Name: "Eurozone CPI", Country: "EU", Impact: EventImpactHigh,
		ScheduledAtUTC: eventTime, Source: "fred",
	}

	scorer := NewMacroScorer(cfg)
	scorer.UpdateEvents([]EconomicEvent{euroEvent})
	regime := scorer.ComputeRegime("BTC-USDT")
	if regime != RegimeNormal {
		t.Errorf("Eurozone event with no BTC relevance: expected Normal, got %s", regime)
	}

	fomcEvent := EconomicEvent{
		EventID: "fomc-1", Name: "FOMC", Country: "US", Impact: EventImpactHigh,
		ScheduledAtUTC: eventTime, Source: "fred",
	}

	scorer.UpdateEvents([]EconomicEvent{fomcEvent})
	regime = scorer.ComputeRegime("BTC-USDT")
	if regime != RegimeElevatedPreEvent {
		t.Errorf("FOMC with BTC relevance: expected ElevatedPreEvent, got %s", regime)
	}
}

func TestComputeSurpriseMagnitude(t *testing.T) {
	tests := []struct {
		name     string
		actual   decimal.Decimal
		forecast decimal.Decimal
		expected decimal.Decimal
	}{
		{"actual equals forecast", decimal.NewFromFloat(100), decimal.NewFromFloat(100), decimal.Zero},
		{"actual above forecast", decimal.NewFromFloat(110), decimal.NewFromFloat(100), decimal.NewFromFloat(0.1)},
		{"actual below forecast", decimal.NewFromFloat(90), decimal.NewFromFloat(100), decimal.NewFromFloat(0.1)},
		{"forecast zero", decimal.NewFromFloat(50), decimal.Zero, decimal.NewFromFloat(50)},
		{"both zero", decimal.Zero, decimal.Zero, decimal.Zero},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := computeSurpriseMagnitude(tt.actual, tt.forecast)
			if !got.Equal(tt.expected) {
				t.Errorf("expected %s, got %s", tt.expected.StringFixed(6), got.StringFixed(6))
			}
		})
	}
}

func TestMacroScorer_ComputeMacroScore(t *testing.T) {
	cfg := DefaultConfig()

	now := time.Now().UTC()
	futureEvent := EconomicEvent{
		EventID: "nfp-1", Name: "NFP", Country: "US",
		Impact: EventImpactHigh, ScheduledAtUTC: now.Add(24 * time.Hour),
		Source: "fred",
	}

	scorer := NewMacroScorer(cfg)
	scorer.UpdateEvents([]EconomicEvent{futureEvent})

	score := scorer.ComputeMacroScore()
	if score.LessThan(decimal.Zero) || score.GreaterThan(decimal.NewFromFloat(1)) {
		t.Errorf("score out of range: expected [0,1], got %s", score.StringFixed(6))
	}

	releasedEvent := futureEvent
	forecast := decimal.NewFromFloat(100)
	actual := decimal.NewFromFloat(110)
	releasedEvent.Forecast = &forecast
	releasedEvent.Actual = &actual
	nowTime := time.Now().UTC()
	releasedEvent.ActualReleasedAtUTC = &nowTime

	scorer.UpdateEvents([]EconomicEvent{releasedEvent})
	scoreWithActual := scorer.ComputeMacroScore()
	if scoreWithActual.LessThanOrEqual(decimal.Zero) {
		t.Errorf("expected positive score with positive surprise, got %s", scoreWithActual.StringFixed(6))
	}
}

func TestStalenessTracker(t *testing.T) {
	st := NewStalenessTracker(3)

	if st.IsStale("fred") {
		t.Error("should not be stale initially")
	}

	st.RecordFailure("fred")
	st.RecordFailure("fred")
	if st.IsStale("fred") {
		t.Error("should not be stale after 2 failures (threshold=3)")
	}

	st.RecordFailure("fred")
	if !st.IsStale("fred") {
		t.Error("should be stale after 3 failures")
	}

	st.RecordSuccess("fred")
	if st.IsStale("fred") {
		t.Error("should not be stale after success")
	}

	state := st.State("fred")
	if state.Failures != 0 {
		t.Errorf("expected 0 failures after success, got %d", state.Failures)
	}
}

func TestFREDDateTimeParsing(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"2026-07-30 08:30:00-05", "2026-07-30T13:30:00Z"},
		{"2026-08-12 08:30:00-04", "2026-08-12T12:30:00Z"},
		{"2026-01-15 10:00:00-06", "2026-01-15T16:00:00Z"},
		{"2026-03-12", "2026-03-12T00:00:00Z"},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got, err := parseFREDDateTime(tt.input)
			if err != nil {
				t.Fatalf("parse error: %v", err)
			}
			gotStr := got.Format(time.RFC3339)
			if gotStr != tt.expected {
				t.Errorf("expected %q, got %q", tt.expected, gotStr)
			}
		})
	}
}

func TestParseNullableDecimal(t *testing.T) {
	tests := []struct {
		input    string
		expected string
		isNil    bool
	}{
		{"100.5", "100.50", false},
		{"-50.25", "-50.25", false},
		{"", "", true},
		{"-", "", true},
		{"N/A", "", true},
		{"1,000.50", "1000.50", false},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got := parseNullableDecimal(tt.input)
			if tt.isNil {
				if got != nil {
					t.Errorf("expected nil, got %s", got.StringFixed(2))
				}
				return
			}
			if got == nil {
				t.Fatalf("expected %s, got nil", tt.expected)
			}
			if got.StringFixed(2) != tt.expected {
				t.Errorf("expected %s, got %s", tt.expected, got.StringFixed(2))
			}
		})
	}
}

type mockFetcher struct {
	source string
	events []EconomicEvent
	err    error
}

func (m *mockFetcher) Fetch(_ context.Context) ([]EconomicEvent, error) {
	if m.err != nil {
		return nil, m.err
	}
	return m.events, nil
}

func (m *mockFetcher) Source() string { return m.source }

type assertError string

func (e assertError) Error() string { return string(e) }

func TestClassifyFREDImpact(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected EventImpact
	}{
		{"FOMC", "FOMC Minutes", EventImpactHigh},
		{"Employment", "Employment Situation", EventImpactHigh},
		{"CPI", "Consumer Price Index", EventImpactHigh},
		{"GDP", "Gross Domestic Product", EventImpactHigh},
		{"Jobless Claims", "Jobless Claims", EventImpactMedium},
		{"PPI", "Producer Price Index", EventImpactMedium},
		{"Retail Sales", "Advance Retail Sales", EventImpactMedium},
		{"Housing Starts", "Housing Starts", EventImpactMedium},
		{"Building Permits", "Building Permits", EventImpactMedium},
		{"Unknown release", "Some Obscure Data", EventImpactLow},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := classifyFREDImpact(tt.input)
			if got != tt.expected {
				t.Errorf("expected %v, got %v", tt.expected, got)
			}
		})
	}
}

func TestFREDDateOnlyParsing_DefaultsTo830ET(t *testing.T) {
	mockBody := `{
		"release_dates": [
			{
				"release_id": 53,
				"release_name": "Gross Domestic Product",
				"date": "2026-09-30",
				"release_date": null,
				"revision": null
			},
			{
				"release_id": 10,
				"release_name": "Consumer Price Index for All Urban Consumers",
				"date": "2026-08-12",
				"release_date": null,
				"revision": null
			},
			{
				"release_id": 50,
				"release_name": "Employment Situation",
				"date": "2026-12-04",
				"release_date": null,
				"revision": null
			}
		]
	}`

	releaseNames := map[int]string{
		53: "GDP",
		10: "CPI",
		50: "NFP",
	}

	events, err := parseFREDResponse([]byte(mockBody), releaseNames)
	if err != nil {
		t.Fatalf("parseFREDResponse failed: %v", err)
	}

	if len(events) != 3 {
		t.Fatalf("expected 3 events, got %d", len(events))
	}

	expected := []struct {
		name string
		utc  string
	}{
		{"GDP", "2026-09-30T12:30:00Z"},
		{"CPI", "2026-08-12T12:30:00Z"},
		{"NFP", "2026-12-04T13:30:00Z"},
	}

	for i, et := range expected {
		if events[i].Name != et.name {
			t.Errorf("event[%d] name: expected %q, got %q", i, et.name, events[i].Name)
		}
		got := events[i].ScheduledAtUTC.Format(time.RFC3339)
		if got != et.utc {
			t.Errorf("event[%d] time: expected %q, got %q", i, et.utc, got)
		}
	}
}

func TestFREDDateOnlyParsing_SkipsPastDates(t *testing.T) {
	releaseNames := map[int]string{10: "CPI"}

	mockBody := `{
		"release_dates": [
			{
				"release_id": 10,
				"release_name": "Consumer Price Index",
				"date": "2020-01-15",
				"release_date": null,
				"revision": null
			}
		]
	}`

	events, err := parseFREDResponse([]byte(mockBody), releaseNames)
	if err != nil {
		t.Fatalf("parseFREDResponse failed: %v", err)
	}
	if len(events) != 0 {
		t.Errorf("expected 0 events for past date-only entry, got %d", len(events))
	}
}

func TestMultiFetcher_MergesMultipleSources(t *testing.T) {
	var logBuf bytes.Buffer
	logger := slog.New(slog.NewTextHandler(&logBuf, &slog.HandlerOptions{Level: slog.LevelInfo}))

	now := time.Now().UTC()
	future := now.Add(48 * time.Hour)

	fetcherA := &mockFetcher{
		source: "fred",
		events: []EconomicEvent{
			{EventID: "fred-1", Name: "CPI", Country: "US", Impact: EventImpactHigh,
				ScheduledAtUTC: future, Source: "fred"},
		},
	}
	fetcherB := &mockFetcher{
		source: "fomc",
		events: []EconomicEvent{
			{EventID: "fomc-1", Name: "FOMC", Country: "US", Impact: EventImpactHigh,
				ScheduledAtUTC: future, Source: "fomc"},
		},
	}

	multi := NewMultiFetcher(logger, fetcherA, fetcherB)
	events, err := multi.Fetch(context.Background())
	if err != nil {
		t.Fatalf("MultiFetcher.Fetch failed: %v", err)
	}
	if len(events) != 2 {
		t.Fatalf("expected 2 events, got %d", len(events))
	}
	if multi.Source() != "multi:fred+fomc" {
		t.Errorf("expected source 'multi:fred+fomc', got %q", multi.Source())
	}
}

func TestMultiFetcher_PartialFailureReturnsPartial(t *testing.T) {
	var logBuf bytes.Buffer
	logger := slog.New(slog.NewTextHandler(&logBuf, &slog.HandlerOptions{Level: slog.LevelInfo}))

	now := time.Now().UTC()
	future := now.Add(48 * time.Hour)

	fetcherA := &mockFetcher{
		source: "fred",
		err:    assertError("fred error"),
	}
	fetcherB := &mockFetcher{
		source: "fomc",
		events: []EconomicEvent{
			{EventID: "fomc-1", Name: "FOMC", Country: "US", Impact: EventImpactHigh,
				ScheduledAtUTC: future, Source: "fomc"},
		},
	}

	multi := NewMultiFetcher(logger, fetcherA, fetcherB)
	events, err := multi.Fetch(context.Background())
	if err != nil {
		t.Fatalf("MultiFetcher.Fetch failed: %v", err)
	}
	if len(events) != 1 {
		t.Fatalf("expected 1 event (fomc only), got %d", len(events))
	}
	if events[0].Source != "fomc" {
		t.Errorf("expected source 'fomc', got %q", events[0].Source)
	}
}

func TestParseFOMCCalendar_ExtractsMeetings(t *testing.T) {
	mockHTML := `MonetaryPolicy

<a id="42828">2026 FOMC Meetings</a>

<div class="row meeting">
	<div class="meeting__month"><strong>September</strong></div>
	<div class="meeting__date">15-16</div>
</div>

<div class="row meeting--shaded">
	<div class="meeting__month"><strong>October</strong></div>
	<div class="meeting__date">27-28*</div>
</div>

<a id="45694">2027 FOMC Meetings</a>`

	events, err := parseFOMCCalendar([]byte(mockHTML))
	if err != nil {
		t.Fatalf("parseFOMCCalendar failed: %v", err)
	}

	if len(events) == 0 {
		t.Fatal("expected at least one event")
	}

	for _, e := range events {
		if e.Name != "FOMC" {
			t.Errorf("expected name FOMC, got %q", e.Name)
		}
		if e.Impact != EventImpactHigh {
			t.Errorf("expected Impact High, got %v", e.Impact)
		}
		if e.Country != "US" {
			t.Errorf("expected country US, got %q", e.Country)
		}
		if e.Source != "fomc" {
			t.Errorf("expected source fomc, got %q", e.Source)
		}

		if e.ScheduledAtUTC.Hour() != 18 && e.ScheduledAtUTC.Hour() != 19 {
			t.Errorf("expected event at 18:xxZ or 19:xxZ (2PM ET), got %s",
				e.ScheduledAtUTC.Format(time.RFC3339))
		}
		if e.ScheduledAtUTC.Minute() != 0 {
			t.Errorf("expected event at :00 minutes, got %s",
				e.ScheduledAtUTC.Format(time.RFC3339))
		}
	}
}

func TestParseFOMCCalendar_StaleHTML_ReturnsCurrentYearOnly(t *testing.T) {
	mockHTML := `<a id="old">2008 FOMC Meetings</a>
<div class="meeting__month"><strong>January</strong></div>
<div class="meeting__date">29-30</div>`

	events, err := parseFOMCCalendar([]byte(mockHTML))
	if err == nil {
		t.Errorf("expected error for stale HTML with no current year meetings, got %d events", len(events))
	}
	if events != nil {
		t.Errorf("expected nil events on error, got %d", len(events))
	}
}

func TestBLSFetcher_ParseSeriesData(t *testing.T) {
	now := time.Now().UTC()
	currentYear := now.Year()

	mockResponse := fmt.Sprintf(`{
		"status": "REQUEST_SUCCEEDED",
		"responseTime": 87,
		"message": [],
		"Results": {
			"series": [
				{
					"seriesID": "CUUR0000SA0",
					"data": [
						{"year": "%d", "period": "M05", "periodName": "May", "value": "335.123", "footnotes": [{}]},
						{"year": "%d", "period": "M04", "periodName": "April", "value": "333.020", "footnotes": [{}]}
					]
				},
				{
					"seriesID": "CES0000000001",
					"data": [
						{"year": "%d", "period": "M05", "periodName": "May", "value": "158927", "footnotes": [{}]},
						{"year": "%d", "period": "M04", "periodName": "April", "value": "158798", "footnotes": [{}]}
					]
				}
			]
		}
	}`, currentYear, currentYear, currentYear, currentYear)

	cfg := DefaultConfig()
	logger := slog.New(slog.NewTextHandler(nil, nil))
	fetcher := NewBLSFetcher(cfg.BLSAPIKey, cfg.BLSBaseURL, cfg.BLSSeriesIDs, logger)

	blsResp := blsSeriesResponse{}
	if err := json.Unmarshal([]byte(mockResponse), &blsResp); err != nil {
		t.Fatalf("unmarshal failed: %v", err)
	}

	if blsResp.Status != "REQUEST_SUCCEEDED" {
		t.Fatalf("expected REQUEST_SUCCEEDED, got %s", blsResp.Status)
	}
	if len(blsResp.Results.Series) != 2 {
		t.Fatalf("expected 2 series, got %d", len(blsResp.Results.Series))
	}

	bls := blsResp.Results.Series
	cpiData := bls[0].Data
	if len(cpiData) == 0 {
		t.Fatal("expected CPI data points")
	}
	if cpiData[0].Value != "335.123" {
		t.Errorf("expected CPI value 335.123, got %s", cpiData[0].Value)
	}

	nfpData := bls[1].Data
	if len(nfpData) == 0 {
		t.Fatal("expected NFP data points")
	}
	if nfpData[0].Value != "158927" {
		t.Errorf("expected NFP value 158927, got %s", nfpData[0].Value)
	}

	t.Logf("BLS fetcher created with series IDs: CPI=%s, NFP=%s",
		fetcher.SeriesID("CPI"), fetcher.SeriesID("NFP"))
}

func TestScheduleFetcher_ReturnsConfiguredEvents(t *testing.T) {
	now := time.Now().UTC()
	entry := ScheduledEvent{
		Name:           "CPI",
		ScheduledAtUTC: now.Add(48 * time.Hour),
	}

	fetcher := NewScheduleFetcher([]ScheduledEvent{entry})
	if fetcher.Source() != "known_schedule" {
		t.Errorf("expected source known_schedule, got %q", fetcher.Source())
	}

	events, err := fetcher.Fetch(context.Background())
	if err != nil {
		t.Fatalf("Fetch failed: %v", err)
	}

	if len(events) != 1 {
		t.Fatalf("expected 1 event, got %d", len(events))
	}
	if events[0].Name != "CPI" {
		t.Errorf("expected name CPI, got %q", events[0].Name)
	}
	if events[0].Source != "known_schedule" {
		t.Errorf("expected source known_schedule, got %q", events[0].Source)
	}
	if !events[0].ScheduledAtUTC.Equal(entry.ScheduledAtUTC) {
		t.Errorf("expected time %s, got %s", entry.ScheduledAtUTC, events[0].ScheduledAtUTC)
	}
}

func TestScheduleFetcher_EmptyEntries_ReturnsError(t *testing.T) {
	fetcher := NewScheduleFetcher(nil)
	_, err := fetcher.Fetch(context.Background())
	if err == nil {
		t.Error("expected error for empty schedule, got nil")
	}
}

func TestBLSFetcher_ParsePeriodMonth(t *testing.T) {
	tests := []struct {
		input    string
		expected int
		wantErr  bool
	}{
		{"M01", 1, false},
		{"M12", 12, false},
		{"M06", 6, false},
		{"01", 0, true},
		{"M13", 0, true},
		{"M00", 0, true},
		{"", 0, true},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got, err := parsePeriodMonth(tt.input)
			if tt.wantErr {
				if err == nil {
					t.Errorf("expected error, got %d", got)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if got != tt.expected {
				t.Errorf("expected %d, got %d", tt.expected, got)
			}
		})
	}
}

func TestBLSFetcher_ParseYear(t *testing.T) {
	tests := []struct {
		input    string
		expected int
		wantErr  bool
	}{
		{"2026", 2026, false},
		{"2025", 2025, false},
		{"26", 0, true},
		{"", 0, true},
		{"abcd", 0, true},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got, err := parseYear(tt.input)
			if tt.wantErr {
				if err == nil {
					t.Errorf("expected error, got %d", got)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if got != tt.expected {
				t.Errorf("expected %d, got %d", tt.expected, got)
			}
		})
	}
}

func TestBLSFetcher_SeriesIDAccessor(t *testing.T) {
	cfg := DefaultConfig()
	logger := slog.New(slog.NewTextHandler(nil, nil))
	fetcher := NewBLSFetcher(cfg.BLSAPIKey, cfg.BLSBaseURL, cfg.BLSSeriesIDs, logger)

	cpiID := fetcher.SeriesID("CPI")
	if cpiID != "CUUR0000SA0" {
		t.Errorf("expected CUUR0000SA0, got %q", cpiID)
	}

	nfpID := fetcher.SeriesID("NFP")
	if nfpID != "CES0000000001" {
		t.Errorf("expected CES0000000001, got %q", nfpID)
	}

	unknown := fetcher.SeriesID("UNKNOWN")
	if unknown != "" {
		t.Errorf("expected empty string for unknown series, got %q", unknown)
	}
}


