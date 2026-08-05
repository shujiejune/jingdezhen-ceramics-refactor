package utils

import (
	"errors"
	"time"

	"github.com/gofiber/fiber/v2"
)

// ErrRangeInvalid is returned by ParseRange for an unparseable/inverted/
// oversize range. Callers map it to HTTP 400.
var ErrRangeInvalid = errors.New("invalid date range")

// MaxRangeDays caps dashboard/audit date ranges (PRD §3.4.2 custom-range
// filter). 366 days allows the `year` preset (365 raw days + up to 1 day from
// the inclusive-`to` normalization).
const MaxRangeDays = 366

// ParseRange resolves a dashboard/audit date-range filter (PRD §3.4.2).
// Supports:
//   - ?range=day|week|month|quarter|year (server computes [now-start, now))
//   - ?from=YYYY-MM-DD&to=YYYY-MM-DD  (explicit; from inclusive, to inclusive)
//
// Defaults to the last 30 days. Returns UTC start-of-day `from` (inclusive)
// and exclusive `to` (start-of-day of to+1, or now if `to` is omitted under a
// preset). Returns ErrRangeInvalid on from>to or >366-day span.
func ParseRange(c *fiber.Ctx) (from, to time.Time, err error) {
	now := time.Now().UTC()
	preset := c.Query("range")
	fromStr := c.Query("from")
	toStr := c.Query("to")

	switch {
	case preset != "":
		from, to, err = presetRange(preset, now)
	case fromStr != "" || toStr != "":
		from, to, err = explicitRange(fromStr, toStr, now)
	default:
		to = now
		from = to.AddDate(0, 0, -30)
	}
	if err != nil {
		return from, to, err
	}

	// Normalize to UTC day boundaries (inclusive from, exclusive to next day).
	from = from.UTC().Truncate(24 * time.Hour)
	if to.IsZero() {
		to = now
	}
	toDay := to.UTC().Truncate(24 * time.Hour)
	if !toDay.Equal(to.UTC()) {
		toDay = toDay.AddDate(0, 0, 1) // include the whole `to` day
	}
	to = toDay

	if !to.After(from) {
		return from, to, ErrRangeInvalid
	}
	if days := int(to.Sub(from) / (24 * time.Hour)); days > MaxRangeDays {
		return from, to, ErrRangeInvalid
	}
	return from, to, nil
}

func presetRange(p string, now time.Time) (time.Time, time.Time, error) {
	to := now
	var from time.Time
	switch p {
	case "day":
		from = to.AddDate(0, 0, -1)
	case "week":
		from = to.AddDate(0, 0, -7)
	case "month":
		from = to.AddDate(0, -1, 0)
	case "quarter":
		from = to.AddDate(0, -3, 0)
	case "year":
		from = to.AddDate(-1, 0, 0)
	default:
		return from, to, ErrRangeInvalid
	}
	return from, to, nil
}

func explicitRange(fromStr, toStr string, now time.Time) (time.Time, time.Time, error) {
	from, err := time.ParseInLocation("2006-01-02", fromStr, time.UTC)
	if err != nil {
		return from, now, ErrRangeInvalid
	}
	var to time.Time
	if toStr == "" {
		to = now
	} else {
		t, err := time.ParseInLocation("2006-01-02", toStr, time.UTC)
		if err != nil {
			return from, now, ErrRangeInvalid
		}
		to = t.AddDate(0, 0, 1) // include the whole to-day (make it exclusive)
	}
	return from, to, nil
}
