package sitemap

import "context"

// Enqueuer is the seam the 4 content services use to trigger a sitemap
// rebuild on publish/unpublish (PRD §4.4). main.go wires a real adapter
// (jobs.Client) in serve mode; tests pass a NoopEnqueuer (or nil). The
// setter on each service accepts this interface (nil-safe).
//
// Keeping it in the sitemap package (not jobs) so the content services
// depend on the SEO concept, not the jobs package — the same decoupling
// pattern as tokenblocklist.TokenRevoker / ratelimit.AttemptTracker.
type Enqueuer interface {
	EnqueueSitemapRebuild(ctx context.Context) error
}

// NoopEnqueuer is a no-op Enqueuer for tests + the worker (which doesn't
// enqueue — it only consumes the sitemap:rebuild job).
type NoopEnqueuer struct{}

// EnqueueSitemapRebuild returns nil (no-op).
func (NoopEnqueuer) EnqueueSitemapRebuild(context.Context) error { return nil }
