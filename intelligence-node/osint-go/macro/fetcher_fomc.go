package macro

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"regexp"
	"strconv"
	"time"
)

type FOMCFetcher struct {
	calendarURL string
	client      *http.Client
	logger      *slog.Logger
}

func NewFOMCFetcher(calendarURL string, logger *slog.Logger) *FOMCFetcher {
	return &FOMCFetcher{
		calendarURL: calendarURL,
		client: &http.Client{
			Timeout: 30 * time.Second,
		},
		logger: logger.With("fetcher", "fomc"),
	}
}

func (f *FOMCFetcher) Source() string { return "fomc" }

func (f *FOMCFetcher) Fetch(ctx context.Context) ([]EconomicEvent, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, f.calendarURL, nil)
	if err != nil {
		return nil, fmt.Errorf("fomc request: %w", err)
	}
	req.Header.Set("User-Agent", "Mozilla/5.0 (X11; Linux x86_64) AppleWebKit/537.36")

	resp, err := f.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("fomc calendar fetch failed: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("fomc read failed: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("fomc non-200 status: %d", resp.StatusCode)
	}

	return parseFOMCCalendar(body)
}

var (
	fomcMonthRE = regexp.MustCompile(`<strong>\s*(January|February|March|April|May|June|July|August|September|October|November|December)\s*</strong>`)
	fomcDateRE  = regexp.MustCompile(`meeting__date[^>]*>\s*(\d{1,2})\s*-\s*(\d{1,2})\*?\s*</div>`)
	months       = map[string]time.Month{
		"January": time.January, "February": time.February, "March": time.March,
		"April": time.April, "May": time.May, "June": time.June,
		"July": time.July, "August": time.August, "September": time.September,
		"October": time.October, "November": time.November, "December": time.December,
	}
)

func parseFOMCCalendar(html []byte) ([]EconomicEvent, error) {
	now := time.Now().UTC()
	currentYear := now.Year()

	htmlStr := string(html)

	yearSection := extractYearSection(htmlStr, currentYear)
	if yearSection == "" {
		yearSection = htmlStr
	}

	monthMatches := fomcMonthRE.FindAllStringSubmatch(yearSection, -1)
	dateMatches := fomcDateRE.FindAllStringSubmatch(yearSection, -1)

	if len(monthMatches) == 0 || len(dateMatches) == 0 {
		return nil, fmt.Errorf("fomc: no meeting dates found for year %d", currentYear)
	}

	count := len(monthMatches)
	if len(dateMatches) < count {
		count = len(dateMatches)
	}

	var events []EconomicEvent
	for i := 0; i < count; i++ {
		monthName := monthMatches[i][1]
		month, ok := months[monthName]
		if !ok {
			continue
		}

		day1, _ := strconv.Atoi(dateMatches[i][1])
		day2, _ := strconv.Atoi(dateMatches[i][2])
		if day1 == 0 || day2 == 0 {
			continue
		}

		eventTime := fomcStatementTime(currentYear, month, day2)
		if eventTime.Before(now) {
			continue
		}

		eventID := fmt.Sprintf("fomc-%s-%s", eventTime.Format("20060102"), eventTime.Format("1504"))

		events = append(events, EconomicEvent{
			EventID:        eventID,
			Name:           "FOMC",
			Country:        "US",
			Impact:         EventImpactHigh,
			ScheduledAtUTC: eventTime,
			Source:         "fomc",
		})
	}

	if len(events) == 0 {
		return nil, fmt.Errorf("fomc: no upcoming meetings found for year %d", currentYear)
	}

	return events, nil
}

func extractYearSection(html string, year int) string {
	pattern := fmt.Sprintf(`<a\s+id="\d+"\s*>\s*%d\s+FOMC\s+Meetings?\s*</a>`, year)
	re := regexp.MustCompile(pattern)
	loc := re.FindStringIndex(html)
	if loc == nil {
		return ""
	}

	start := loc[1]

	nextPanelRE := regexp.MustCompile(`<div\s+class="panel\s+panel-default"`)
	nextLoc := nextPanelRE.FindStringIndex(html[start:])
	if nextLoc != nil {
		return html[start : start+nextLoc[0]]
	}

	return html[start:]
}

func fomcStatementTime(year int, month time.Month, day int) time.Time {
	offset := fomcEasternOffset(year, month, day)
	loc := time.FixedZone(fmt.Sprintf("ET%+d", offset), offset*3600)
	return time.Date(year, month, day, 14, 0, 0, 0, loc).UTC()
}

func fomcEasternOffset(year int, month time.Month, day int) int {
	if month >= time.April && month <= time.October {
		return -4
	}
	if month == time.March && day >= 14 {
		return -4
	}
	if month == time.November && day <= 7 {
		return -4
	}
	return -5
}
