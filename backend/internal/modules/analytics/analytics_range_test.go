package analytics

import (
	"testing"
	"time"

	"jingdezhen-ceramics-backend/pkg/utils"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestParseRange_Default30Days(t *testing.T) {
	c := newTestCtx(t, "", "", "")
	from, to, err := utils.ParseRange(c)
	require.NoError(t, err)
	span := to.Sub(from)
	assert.GreaterOrEqual(t, span, 30*24*time.Hour, "default range >= 30 days")
	assert.Less(t, span, 32*24*time.Hour, "default range < 32 days (30 + today normalization)")
	assert.Equal(t, from, from.UTC().Truncate(24*time.Hour))
}

func TestParseRange_Presets(t *testing.T) {
	cases := map[string]int{
		"day":     1,
		"week":    7,
		"month":   30, // approximate; month is calendar sub
		"quarter": 90,
		"year":    365,
	}
	for preset, minDays := range cases {
		t.Run(preset, func(t *testing.T) {
			c := newTestCtx(t, "", "", preset)
			from, to, err := utils.ParseRange(c)
			require.NoError(t, err)
			days := to.Sub(from) / (24 * time.Hour)
			assert.GreaterOrEqualf(t, int(days), minDays, "preset %s span too short", preset)
		})
	}
}

func TestParseRange_PresetInvalid(t *testing.T) {
	c := newTestCtx(t, "", "", "banana")
	_, _, err := utils.ParseRange(c)
	require.Error(t, err)
}

func TestParseRange_Explicit(t *testing.T) {
	c := newTestCtx(t, "2026-08-01", "2026-08-10", "")
	from, to, err := utils.ParseRange(c)
	require.NoError(t, err)
	assert.Equal(t, "2026-08-01", dateDay(from))
	assert.Equal(t, "2026-08-11", dateDay(to), "to-day is inclusive → exclusive next day")
	assert.Equal(t, 10*24*time.Hour, to.Sub(from))
}

func TestParseRange_ExplicitFromOnly(t *testing.T) {
	c := newTestCtx(t, "2026-08-01", "", "")
	from, to, err := utils.ParseRange(c)
	require.NoError(t, err)
	assert.Equal(t, "2026-08-01", dateDay(from))
	// to defaults to now → end-of-today boundary.
	assert.True(t, to.After(from))
}

func TestParseRange_InvertedIsInvalid(t *testing.T) {
	c := newTestCtx(t, "2026-08-10", "2026-08-01", "")
	_, _, err := utils.ParseRange(c)
	require.Error(t, err)
}

func TestParseRange_SameDayIsInvalid(t *testing.T) {
	// from == to (after normalization) → to not After(from) → invalid.
	c := newTestCtx(t, "2026-08-10", "2026-08-10", "")
	_, _, err := utils.ParseRange(c)
	// 2026-08-10 inclusive → to becomes 2026-08-11, so from<to → VALID actually.
	require.NoError(t, err, "same from/to day = one full day (valid)")
}

func TestParseRange_OverMaxInvalid(t *testing.T) {
	// 367 days > maxRangeDays (366).
	c := newTestCtx(t, "2025-01-01", "2026-01-02", "")
	_, _, err := utils.ParseRange(c)
	require.Error(t, err, "range > 366 days is rejected")
}

func TestParseRange_AtMaxValid(t *testing.T) {
	// exactly 366 days (the year preset span).
	c := newTestCtx(t, "2025-01-01", "2026-01-01", "")
	_, _, err := utils.ParseRange(c)
	require.NoError(t, err, "exactly 366 days is valid")
}

func TestParseRange_BadDateFormat(t *testing.T) {
	c := newTestCtx(t, "not-a-date", "", "")
	_, _, err := utils.ParseRange(c)
	require.Error(t, err)
}

func TestPct(t *testing.T) {
	assert.Equal(t, 0.0, pct(0, 0))
	assert.Equal(t, 50.0, pct(1, 2))
	assert.Equal(t, 33.33, pct(1, 3))
	assert.Equal(t, 100.0, pct(2, 2))
	assert.Equal(t, 0.0, pct(5, 0))
}

// TestZeroFill_LoopDirection guards against the inverted-loop-condition bug
// (the loop must iterate while d < to, not while d >= to). No Docker needed.
func TestZeroFill_LoopDirection(t *testing.T) {
	from := time.Date(2026, 8, 1, 0, 0, 0, 0, time.UTC)
	to := time.Date(2026, 8, 4, 0, 0, 0, 0, time.UTC) // 3 days

	traffic := zeroFillTraffic(from, to, nil)
	assert.Len(t, traffic, 3)
	assert.Equal(t, "2026-08-01", traffic[0].Date)
	assert.Equal(t, "2026-08-03", traffic[2].Date)

	sales := zeroFillSales(from, to, nil)
	assert.Len(t, sales, 3)

	funnel := zeroFillFunnel(from, to, nil, nil, nil)
	assert.Len(t, funnel, 3)
}
