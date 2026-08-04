package pdf

import "context"

// NoopGenerator is the dev/test default (PDF_MODE=local). Render returns
// ErrPDFUnavailable so the worker skips storage + pdf_key stays NULL; the
// download endpoint 404s gracefully. Dev + unit tests never need the sidecar.
type NoopGenerator struct{}

func NewNoopGenerator() *NoopGenerator { return &NoopGenerator{} }

func (NoopGenerator) Mode() string { return "local" }

func (NoopGenerator) Render(_ context.Context, _ RenderRequest) ([]byte, error) {
	return nil, ErrPDFUnavailable
}
