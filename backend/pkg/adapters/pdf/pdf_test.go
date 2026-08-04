package pdf_test

import (
	"context"
	"errors"
	"testing"

	"jingdezhen-ceramics-backend/pkg/adapters/pdf"

	"github.com/stretchr/testify/require"
)

// TestNoopGenerator_ReturnsErrPDFUnavailable verifies the dev default skips
// storage: the worker treats ErrPDFUnavailable as "local mode, no PDF" and
// leaves pdf_key NULL. No sidecar is needed for this test.
func TestNoopGenerator_ReturnsErrPDFUnavailable(t *testing.T) {
	g := pdf.NewNoopGenerator()
	require.Equal(t, "local", g.Mode())

	_, err := g.Render(context.Background(), pdf.RenderRequest{HTML: "<p>x</p>"})
	require.ErrorIs(t, err, pdf.ErrPDFUnavailable)
}

// TestNoopGenerator_ErrIsSentinel verifies ErrPDFUnavailable is a distinct
// sentinel (the worker must not retry it as a chromedp failure — local mode
// is a permanent skip, not a transient error).
func TestNoopGenerator_ErrIsSentinel(t *testing.T) {
	g := pdf.NewNoopGenerator()
	_, err := g.Render(context.Background(), pdf.RenderRequest{})
	require.True(t, errors.Is(err, pdf.ErrPDFUnavailable))
	require.False(t, errors.Is(err, context.DeadlineExceeded),
		"ErrPDFUnavailable must not be confused with a chromedp timeout")
}

// TestChromedpGenerator_EmptyURL verifies the live impl surfaces a clear error
// when CHROMEDP_URL is unset (misconfiguration), rather than attempting a
// nil-context connection. Does NOT connect to a sidecar.
func TestChromedpGenerator_EmptyURL(t *testing.T) {
	g := pdf.NewChromedpGenerator("")
	require.Equal(t, "chromedp", g.Mode())
	_, err := g.Render(context.Background(), pdf.RenderRequest{HTML: "<p>x</p>"})
	require.Error(t, err, "empty CHROMEDP_URL must error before any connection")
}
