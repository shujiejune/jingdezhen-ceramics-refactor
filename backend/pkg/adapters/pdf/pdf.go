// Package pdf defines the PDF-generation adapter (TDD §12: chromedp HTML→PDF).
//
// One engine serves all three branded documents (certificate, itinerary,
// quote). The adapter takes a rendered HTML string + print options and returns
// PDF bytes — it does NOT know about certificates/quotes; the caller builds the
// HTML from a template (see internal/platform/pdftmpl) and the worker stores
// the result via the storage adapter (pkg/adapters/storage).
//
// Two implementations, selected by PDF_MODE (the same env-flip convention as
// payments/storage, TDD §4.1):
//   - NoopGenerator (PDF_MODE=local, dev default): returns ErrPDFUnavailable so
//     the worker skips storage and pdf_key stays NULL. Dev + tests never need
//     the sidecar.
//   - ChromedpGenerator (PDF_MODE=chromedp, prod): connects to a headless-shell
//     sidecar (chromedp/headless-shell) over the DevTools remote protocol via
//     chromedp.NewRemoteAllocator. The Go binary never bundles Chromium.
package pdf

import (
	"context"
	"errors"
)

// ErrPDFUnavailable is returned by NoopGenerator (local mode). The worker
// treats it as "skip storage" — pdf_key stays NULL and the download endpoint
// 404s gracefully. Distinct from a chromedp failure (which the worker retries).
var ErrPDFUnavailable = errors.New("pdf generation is disabled in local mode")

// Generator is the contract a PDF provider satisfies. Services + the worker
// depend on this interface, never on a concrete client (TDD §4.1/§12).
type Generator interface {
	// Mode returns "local" (noop) or "chromedp" — used for logging + tests.
	Mode() string
	// Render prints the given HTML to a PDF + returns the bytes. The HTML must
	// be self-contained (inline CSS; any <img> fetched over the network at
	// render time, so chromedp must reach the asset URL).
	Render(ctx context.Context, req RenderRequest) ([]byte, error)
}

// RenderRequest is the input to Render. The HTML is a complete document
// (<!DOCTYPE html>…); the print options map to chromedp's page.PrintToPDF.
type RenderRequest struct {
	HTML         string
	PaperFormat  string // "A4" (default) | "Letter"
	Landscape    bool
	MarginTop    float64 // inches; 0 = use the chromedp default (~0.4)
	MarginBottom float64
	MarginLeft   float64
	MarginRight  float64
}
