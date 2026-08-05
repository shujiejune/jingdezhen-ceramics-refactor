package analytics

import (
	"testing"

	"github.com/gofiber/fiber/v2"
	"github.com/valyala/fasthttp"
)

// newTestCtx builds a *fiber.Ctx with the given query string, for handler-level
// unit tests that parse query params (parseRange). It uses a throwaway Fiber
// app + fasthttp ctx; the ctx is valid only for the lifetime of the test.
func newTestCtx(t *testing.T, from, to, preset string) *fiber.Ctx {
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
