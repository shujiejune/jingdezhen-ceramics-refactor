// Package jobs is the thin Asynq wrapper for the platform's background job
// queue (TDD §4.2). One Go binary, two run modes: `serve` calls only NewClient
// (enqueue jobs from request handlers / webhooks); `worker` calls RunServer
// (process jobs). Both point at the same Redis (REDIS_URL).
//
// Job types are declared as exported constants so enqueue side and handler
// side share one source of truth. A handler is registered per type; until a
// feature module supplies real logic, the handler is a no-op that logs — this
// keeps the worker runnable end-to-end from day one.
package jobs

import (
	"context"
	"encoding/json"
	"fmt"
	"log"

	"github.com/hibiken/asynq"
)

// Job type strings. These are the values Asynq stores in Redis; renaming them
// breaks in-flight jobs, so keep them stable.
const (
	TypeEmailSend       = "email:send"
	TypePaymentFinalize = "payment:finalize"
	TypeFXRefresh       = "fx:refresh"
	TypeMediaTranscode  = "media:transcode"
	TypePDFGenerate     = "pdf:generate"
	TypeSitemapRebuild  = "sitemap:rebuild"
	TypeAnalyticsRollup = "analytics:rollup"
	TypeStockCheck      = "stock:check"
	TypeSLACheck        = "sla:check"
)

// --- Payloads ---------------------------------------------------------------

// EmailSendPayload is the body for TypeEmailSend. All Brevo sends enqueue
// this so a flaky email API never blocks a request (TDD §4.2, retry ×5).
type EmailSendPayload struct {
	To        string `json:"to"`
	Subject   string `json:"subject"`
	PlainText string `json:"plain_text,omitempty"`
	HTML      string `json:"html,omitempty"`
	Template  string `json:"template,omitempty"` // named Brevo template id, future
}

// PaymentFinalizePayload is the body for TypePaymentFinalize, enqueued by
// the webhook handler after signature verification. Idempotent by GatewayRef,
// or by the mock seam in dev (payments_mode=mock).
type PaymentFinalizePayload struct {
	OrderID          int64  `json:"order_id"`           // drives order.Status created→paid (0 for a deposit)
	ItineraryQuoteID int64  `json:"itinerary_quote_id"` // drives itinerary quote sent→deposit_paid (0 for an order)
	Success          bool   `json:"success"`            // true → MarkPaid / MarkDepositPaid; false → (TBD) cancel
	Gateway          string `json:"gateway,omitempty"`  // "airwallex" | "paypal" | "mock"
	GatewayRef       string `json:"gateway_ref,omitempty"`
}

// FXRefreshPayload triggers an FX-rate refresh (ECB fetch → 2% markup → upsert).
// Empty by design; the cron-scheduled instance uses defaults.
type FXRefreshPayload struct{}

// StockCheckPayload triggers a low-stock alert check for the SKUs touched by a
// paid order (TDD line 234: "order paid | fires low-stock notification/email").
// Enqueued by order.MarkPaid; the worker queries each SKU's post-decrement
// stock + fires dashboard notifications + Brevo emails to ecommerce operators.
type StockCheckPayload struct {
	OrderID int64   `json:"order_id"`
	SkuIDs  []int64 `json:"sku_ids"`
}

// PDFGeneratePayload is the body for TypePDFGenerate (TDD §12). Enqueued by the
// certificate service at issue/regenerate (and the itinerary quote service at
// confirm). The worker renders the doc via the chromedp adapter + stores the
// result via the storage adapter, populating the entity's pdf_key.
type PDFGeneratePayload struct {
	Kind     string `json:"kind"`      // "certificate" | "itinerary_quote"
	EntityID int64  `json:"entity_id"` // cert id or quote id
	Locale   string `json:"locale,omitempty"`
}

// --- Enqueue helpers (used by the `serve` mode) -----------------------------

// Client wraps asynq.Client for enqueuing jobs. Services depend on this to
// defer flaky/heavy work out of the request path.
type Client struct {
	ac *asynq.Client
}

// NewClient builds an enqueue client backed by the given Redis address.
// redisAddr is "host:port"; for a redis://... URL parse it down to host:port.
func NewClient(redisAddr string) *Client {
	return &Client{ac: asynq.NewClient(asynq.RedisClientOpt{Addr: redisAddr})}
}

// Close releases the underlying Asynq client. Call on shutdown.
func (c *Client) Close() error { return c.ac.Close() }

// enqueue is a small helper: build a task from JSON and Enqueue it with opts.
func (c *Client) enqueue(ctx context.Context, typ string, payload any, opts ...asynq.Option) error {
	body, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("jobs: marshal %s payload: %w", typ, err)
	}
	task := asynq.NewTask(typ, body)
	if _, err := c.ac.EnqueueContext(ctx, task, opts...); err != nil {
		return fmt.Errorf("jobs: enqueue %s: %w", typ, err)
	}
	return nil
}

// EnqueueEmailSend schedules a transactional email. Default retry ×5.
func (c *Client) EnqueueEmailSend(ctx context.Context, p EmailSendPayload) error {
	return c.enqueue(ctx, TypeEmailSend, p, asynq.MaxRetry(5))
}

// EnqueuePaymentFinalize enqueues order finalization from a webhook. Retried
// with backoff; idempotency is the handler's responsibility (by GatewayRef).
func (c *Client) EnqueuePaymentFinalize(ctx context.Context, p PaymentFinalizePayload) error {
	return c.enqueue(ctx, TypePaymentFinalize, p, asynq.MaxRetry(5))
}

// EnqueueFXRefresh enqueues a one-off FX refresh (used by tests / manual trigger).
func (c *Client) EnqueueFXRefresh(ctx context.Context) error {
	return c.enqueue(ctx, TypeFXRefresh, FXRefreshPayload{})
}

// EnqueueStockCheck enqueues a low-stock alert check for a paid order's SKUs
// (TDD line 234). Retried ×5 — a flaky alert shouldn't be silently lost.
func (c *Client) EnqueueStockCheck(ctx context.Context, p StockCheckPayload) error {
	return c.enqueue(ctx, TypeStockCheck, p, asynq.MaxRetry(5))
}

// EnqueuePDFGenerate enqueues a PDF render (TDD §12). Retried ×5 — a flaky
// chromedp sidecar shouldn't lose the doc; the render is idempotent (overwrites
// the prior pdf_key).
func (c *Client) EnqueuePDFGenerate(ctx context.Context, p PDFGeneratePayload) error {
	return c.enqueue(ctx, TypePDFGenerate, p, asynq.MaxRetry(5))
}

// --- Server / worker (the `worker` mode) -------------------------------------

// Server wraps the Asynq server + mux with the platform's job types.
// Assign handlers to the exported fields, then call Run.
// Think of the mux as a background jobs router.
// It matches the job label against a list of registered routes,
// hands the job to the correct handler.
//
//	srv := jobs.NewServer(addr)
//	srv.EmailSend = myBrevoEmailHandler
//	srv.PaymentFinalize = myOrderFinalizer
//	srv.Run(ctx)  // blocks until ctx is cancelled
//
// A nil handler => the job is logged and treated as success, so the worker
// runs end-to-end before any feature module supplies real logic.
type Server struct {
	srv *asynq.Server
	mux *asynq.ServeMux

	// Handlers. Set these before calling Run; they are bound to the mux at
	// Run time, so reassigning afterwards has no effect on in-flight workers.
	EmailSend       func(context.Context, EmailSendPayload) error
	PaymentFinalize func(context.Context, PaymentFinalizePayload) error
	FXRefresh       func(context.Context) error
	SLACheck        func(context.Context) error
	AnalyticsRollup func(context.Context) error
	MediaTranscode  func(context.Context) error
	PDFGenerate     func(context.Context, PDFGeneratePayload) error
	SitemapRebuild  func(context.Context) error
	StockCheck      func(context.Context, StockCheckPayload) error
}

// NewServer constructs the Asynq worker server. Concurrency defaults to a
// conservative 10 (single VPS, MVP-sized); tune via env when load demands it.
func NewServer(redisAddr string) *Server {
	srv := asynq.NewServer(
		asynq.RedisClientOpt{Addr: redisAddr},
		asynq.Config{
			// Conservative 10 (single VPS, MVP-sized); tune via env when load demands.
			// Asynq's graceful shutdown (srv.Shutdown in Run) drains in-flight jobs.
			Concurrency: 10,
		},
	)
	return &Server{srv: srv, mux: asynq.NewServeMux()}
}

// Run binds the current handlers to the mux and starts the worker. Blocks
// until ctx is cancelled, at which point it triggers Asynq's graceful shutdown.
func (s *Server) Run(ctx context.Context) error {
	s.mux.HandleFunc(TypeEmailSend, func(_ context.Context, t *asynq.Task) error {
		var p EmailSendPayload
		if err := json.Unmarshal(t.Payload(), &p); err != nil {
			return fmt.Errorf("jobs: unmarshal %s: %w", TypeEmailSend, err)
		}
		return runHandler(s.EmailSend, "email:send", p)
	})
	s.mux.HandleFunc(TypePaymentFinalize, func(_ context.Context, t *asynq.Task) error {
		var p PaymentFinalizePayload
		if err := json.Unmarshal(t.Payload(), &p); err != nil {
			return fmt.Errorf("jobs: unmarshal %s: %w", TypePaymentFinalize, err)
		}
		return runHandler(s.PaymentFinalize, "payment:finalize", p)
	})
	s.mux.HandleFunc(TypeFXRefresh, func(_ context.Context, _ *asynq.Task) error {
		return runNoop(s.FXRefresh, "fx:refresh")
	})
	s.mux.HandleFunc(TypeSLACheck, func(_ context.Context, _ *asynq.Task) error {
		return runNoop(s.SLACheck, "sla:check")
	})
	s.mux.HandleFunc(TypeAnalyticsRollup, func(_ context.Context, _ *asynq.Task) error {
		return runNoop(s.AnalyticsRollup, "analytics:rollup")
	})
	s.mux.HandleFunc(TypeMediaTranscode, func(_ context.Context, _ *asynq.Task) error {
		return runNoop(s.MediaTranscode, "media:transcode")
	})
	s.mux.HandleFunc(TypePDFGenerate, func(_ context.Context, t *asynq.Task) error {
		var p PDFGeneratePayload
		if err := json.Unmarshal(t.Payload(), &p); err != nil {
			return fmt.Errorf("jobs: unmarshal %s: %w", TypePDFGenerate, err)
		}
		if s.PDFGenerate == nil {
			return nil // no-op when not wired (e.g. serve mode)
		}
		return s.PDFGenerate(context.Background(), p)
	})
	s.mux.HandleFunc(TypeSitemapRebuild, func(_ context.Context, _ *asynq.Task) error {
		return runNoop(s.SitemapRebuild, "sitemap:rebuild")
	})
	s.mux.HandleFunc(TypeStockCheck, func(_ context.Context, t *asynq.Task) error {
		var p StockCheckPayload
		if err := json.Unmarshal(t.Payload(), &p); err != nil {
			return fmt.Errorf("jobs: unmarshal %s: %w", TypeStockCheck, err)
		}
		if s.StockCheck == nil {
			return nil // no-op when not wired (e.g. serve mode)
		}
		return s.StockCheck(context.Background(), p)
	})

	errCh := make(chan error, 1)
	go func() { errCh <- s.srv.Run(s.mux) }()
	select {
	case <-ctx.Done():
		s.srv.Shutdown()
		return nil
	case err := <-errCh:
		return err
	}
}

// runHandler invokes a payload handler if non-nil, else no-ops with a log.
// The `name` is for the no-op log line.
func runHandler[P any](h func(context.Context, P) error, name string, p P) error {
	if h == nil {
		log.Printf("[jobs] %s no-op", name)
		return nil
	}
	return h(context.Background(), p)
}

// runNoop invokes a no-arg handler if non-nil, else no-ops with a log.
func runNoop(h func(context.Context) error, name string) error {
	if h == nil {
		log.Printf("[jobs] %s no-op", name)
		return nil
	}
	return h(context.Background())
}

// Scheduler runs cron-based jobs. Asynq keeps scheduled tasks in Redis, so the
// scheduler can run alongside the worker (or as its own process) and survive
// restarts — but for the single-VPS MVP one scheduler instance in worker mode
// is enough. TDD §4.2 crons: fx:refresh daily, sla:check 15min, analytics nightly.
type Scheduler struct {
	sch *asynq.Scheduler
}

// NewScheduler builds the cron scheduler with the platform's recurring jobs.
// Register returns an error only on a malformed cron spec — a programmer error,
// so we panic (fail fast at startup rather than silently skipping a job).
func NewScheduler(redisAddr string) *Scheduler {
	sch := asynq.NewScheduler(asynq.RedisClientOpt{Addr: redisAddr}, nil)
	mustRegister := func(spec string, task *asynq.Task) {
		if _, err := sch.Register(spec, task); err != nil {
			panic(fmt.Errorf("jobs.NewScheduler: bad cron spec %q for %s: %w", spec, task.Type(), err))
		}
	}
	// fx:refresh daily at 16:05 CET (after ECB publishes ~16:00 CET).
	mustRegister("* 5 16 * *", asynq.NewTask(TypeFXRefresh, nil))
	// sla:check every 15 minutes.
	mustRegister("*/15 * * * *", asynq.NewTask(TypeSLACheck, nil))
	// analytics:rollup nightly at 00:05.
	mustRegister("5 0 * * *", asynq.NewTask(TypeAnalyticsRollup, nil))
	return &Scheduler{sch: sch}
}

// Run starts the scheduler and blocks until ctx is cancelled.
func (s *Scheduler) Run(ctx context.Context) error {
	errCh := make(chan error, 1)
	go func() { errCh <- s.sch.Run() }()
	select {
	case <-ctx.Done():
		s.sch.Shutdown()
		return nil
	case err := <-errCh:
		return err
	}
}
