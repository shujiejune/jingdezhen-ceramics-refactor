package pdf

import (
	"context"
	"fmt"
	"net/url"
	"time"

	"github.com/chromedp/cdproto/page"
	"github.com/chromedp/chromedp"
)

// ChromedpGenerator renders HTML→PDF via a headless-shell sidecar over the
// DevTools remote protocol (TDD §12). The Go binary never bundles Chromium;
// PDF_MODE=chromedp connects to ws://chromedp:9222 (the compose sidecar). A
// connection is made per Render (no long-lived browser) so a sidecar restart
// is self-healing.
type ChromedpGenerator struct {
	remoteURL string // ws://chromedp:9222
}

func NewChromedpGenerator(remoteURL string) *ChromedpGenerator {
	return &ChromedpGenerator{remoteURL: remoteURL}
}

func (ChromedpGenerator) Mode() string { return "chromedp" }

// paperSize maps PaperFormat → inches (chromedp's page.PrintToPDF uses inches).
// A4 = 8.27×11.69; Letter = 8.5×11. Empty/unknown → A4 (default).
func paperSize(format string) (w, h float64) {
	switch format {
	case "Letter":
		return 8.5, 11
	default: // "A4" or empty
		return 8.27, 11.69
	}
}

// Render navigates the sidecar to a data: URL carrying the HTML (self-contained;
// no temp file) + prints to PDF. A 30s cap guards against a hung sidecar.
func (g *ChromedpGenerator) Render(ctx context.Context, req RenderRequest) ([]byte, error) {
	if g.remoteURL == "" {
		return nil, fmt.Errorf("pdf.ChromedpGenerator: CHROMEDP_URL not configured")
	}
	ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()

	// Connect to the remote headless-shell. NewRemoteAllocator returns a new
	// context; a tab is created on the first chromedp.Run target action.
	allocCtx, allocCancel := chromedp.NewRemoteAllocator(ctx, g.remoteURL)
	defer allocCancel()

	taskCtx, taskCancel := chromedp.NewContext(allocCtx)
	defer taskCancel()

	// Encode the HTML as a data: URL so chromedp loads it directly (no temp
	// file, no HTTP server). The base64 path avoids URL-length + escaping issues
	// for large templates.
	dataURL := "data:text/html;charset=utf-8," + url.PathEscape(req.HTML)

	paperW, paperH := paperSize(req.PaperFormat)
	print := page.PrintToPDF().
		WithLandscape(req.Landscape).
		WithPaperWidth(paperW).
		WithPaperHeight(paperH).
		WithPrintBackground(true). // brand colors render
		WithDisplayHeaderFooter(false)
	if req.MarginTop > 0 {
		print = print.WithMarginTop(req.MarginTop)
	}
	if req.MarginBottom > 0 {
		print = print.WithMarginBottom(req.MarginBottom)
	}
	if req.MarginLeft > 0 {
		print = print.WithMarginLeft(req.MarginLeft)
	}
	if req.MarginRight > 0 {
		print = print.WithMarginRight(req.MarginRight)
	}

	var buf []byte
	if err := chromedp.Run(taskCtx,
		chromedp.Navigate(dataURL),
		chromedp.ActionFunc(func(ctx context.Context) error {
			var err error
			buf, _, err = print.Do(ctx)
			return err
		}),
	); err != nil {
		return nil, fmt.Errorf("pdf.ChromedpGenerator.Render: %w", err)
	}
	return buf, nil
}