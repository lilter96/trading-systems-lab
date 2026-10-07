package macro

import (
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/shopspring/decimal"
)

type fredReleaseDatesResponse struct {
	ReleaseDates []fredReleaseDate `json:"release_dates"`
}

type fredReleaseDate struct {
	ReleaseID   int    `json:"release_id"`
	ReleaseName string `json:"release_name"`
	Date        string `json:"date"`
	ReleaseDate string `json:"release_date"`
	Revision    bool   `json:"revision"`
}

func parseFREDResponse(body []byte, releaseNames map[int]string) ([]EconomicEvent, error) {
	var resp fredReleaseDatesResponse
	if err := json.Unmarshal(body, &resp); err != nil {
		return nil, fmt.Errorf("fred parse error: %w", err)
	}

	var events []EconomicEvent
	now := time.Now().UTC()

	for _, rd := range resp.ReleaseDates {
		if rd.Revision {
			continue
		}

		var parsed time.Time
		var err error

		if rd.ReleaseDate != "" && rd.ReleaseDate != "N/A" {
			parsed, err = parseFREDDateTime(rd.ReleaseDate)
		} else if rd.Date != "" {
			parsed, err = defaultFREDDateTime(rd.Date)
		} else {
			continue
		}
		if err != nil {
			continue
		}

		if parsed.Before(now) {
			continue
		}

		eventName := releaseNames[rd.ReleaseID]
		if eventName == "" {
			eventName = rd.ReleaseName
		}

		eventID := fmt.Sprintf("fred-%d-%s", rd.ReleaseID, parsed.Format("20060102"))

		events = append(events, EconomicEvent{
			EventID:        eventID,
			Name:           eventName,
			Country:        "US",
			Impact:         classifyFREDImpact(rd.ReleaseName),
			ScheduledAtUTC: parsed,
			Source:         "fred",
		})
	}

	return events, nil
}

func defaultFREDDateTime(dateStr string) (time.Time, error) {
	offset := easternOffset(dateStr)
	timeStr := fmt.Sprintf("%s 08:30:00%+03d00", dateStr, offset)
	return parseFREDDateTime(timeStr)
}

func easternOffset(dateStr string) int {
	date, err := time.Parse("2006-01-02", dateStr)
	if err != nil {
		return -4
	}
	month := date.Month()
	if month >= time.April && month <= time.October {
		return -4
	}
	if month == time.March && date.Day() >= 14 {
		return -4
	}
	if month == time.November && date.Day() <= 7 {
		return -4
	}
	return -5
}

func parseFREDDateTime(raw string) (time.Time, error) {
	layouts := []string{
		"2006-01-02 15:04:05-07",
		"2006-01-02 15:04:05-0700",
		"2006-01-02",
	}

	for _, layout := range layouts {
		t, err := time.Parse(layout, raw)
		if err == nil {
			return t.UTC(), nil
		}
	}

	return time.Time{}, fmt.Errorf("cannot parse FRED date time: %s", raw)
}

func classifyFREDImpact(releaseName string) EventImpact {
	upper := strings.ToUpper(releaseName)
	highImpact := []string{"FOMC", "EMPLOYMENT", "CPI", "CONSUMER PRICE INDEX",
		"GDP", "GROSS DOMESTIC PRODUCT", "NFP", "NON-FARM",
		"INTEREST RATE", "FED", "BENCHMARK REVISION"}
	mediumImpact := []string{"PPI", "PRODUCER PRICE INDEX", "RETAIL SALES",
		"INDUSTRIAL PRODUCTION", "CONSUMER SENTIMENT", "DURABLE GOODS",
		"HOUSING STARTS", "BUILDING PERMITS", "TRADE BALANCE",
		"WHOLESALE INVENTORIES", "ISM MANUFACTURING", "ISM SERVICES",
		"JOBLESS CLAIMS"}

	for _, keyword := range highImpact {
		if strings.Contains(upper, keyword) {
			return EventImpactHigh
		}
	}
	for _, keyword := range mediumImpact {
		if strings.Contains(upper, keyword) {
			return EventImpactMedium
		}
	}
	return EventImpactLow
}

func parseNullableDecimal(s string) *decimal.Decimal {
	s = strings.TrimSpace(s)
	if s == "" || s == "-" || s == "N/A" {
		return nil
	}
	s = strings.ReplaceAll(s, ",", "")
	d, err := decimal.NewFromString(s)
	if err != nil {
		return nil
	}
	return &d
}
