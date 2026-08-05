package utils

import (
	"testing"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/valyala/fasthttp"
)

func newCtx(t *testing.T, from, to, preset string) *fiber.Ctx {
	t.Helper()
	app := fiber.New()
	ctx := app.AcquireCtx(&fasthttp.RequestCtx{})
	q := ""
	if from != "" {
		q += "from=" + from + "&"
	}
	if to != "" {
		q += "to=" + to + "&"
	}
	if preset != "" {
		q += "range=" + preset + "&"
	}
	ctx.Request().URI().SetQueryString(q)
	return ctx
}

func TestParseRange_Default30Days(t *testing.T) {
	c := newCtx(t, "", "", "")
	from, to, err := ParseRange(c)
	require.NoError(t, err)
	span := to.Sub(from)
	assert.GreaterOrEqual(t, span, 30*24*time.Hour)
	assert.Less(t, span, 32*24*time.Hour)
}

func TestParseRange_Presets(t *testing.T) {
	cases := []string{"day", "week", "month", "quarter", "year"}
	for _, preset := range cases {
		t.Run(preset, func(t *testing.T) {
			c := newCtx(t, "", "", preset)
			_, _, err := ParseRange(c)
			require.NoError(t, err)
		})
	}
}

func TestParseRange_PresetInvalid(t *testing.T) {
	c := newCtx(t, "", "", "banana")
	_, _, err := ParseRange(c)
	require.Error(t, err)
}

func TestParseRange_Explicit(t *testing.T) {
	c := newCtx(t, "2026-08-01", "2026-08-10", "")
	from, to, err := ParseRange(c)
	require.NoError(t, err)
	assert.Equal(t, "2026-08-01", from.UTC().Format("2006-01-02"))
	assert.Equal(t, "2026-08-11", to.UTC().Format("2006-01-02"), "to-day inclusive → exclusive next day")
}

func TestParseRange_InvertedInvalid(t *testing.T) {
	c := newCtx(t, "2026-08-10", "2026-08-01", "")
	_, _, err := ParseRange(c)
	require.Error(t, err)
}

func TestParseRange_BadDateFormat(t *testing.T) {
	c := newCtx(t, "not-a-date", "", "")
	_, _, err := ParseRange(c)
	require.Error(t, err)
}