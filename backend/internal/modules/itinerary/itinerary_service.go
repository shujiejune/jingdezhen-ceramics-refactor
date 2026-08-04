package itinerary

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"regexp"
	"strconv"
	"time"

	"jingdezhen-ceramics-backend/internal/models"
	"jingdezhen-ceramics-backend/internal/platform/i18ncontent"

	"github.com/shopspring/decimal"
)

// --- Injected dependencies (narrow interfaces, satisfied by existing services) ---

// EmailEnqueuer enqueues the 24h-SLA acknowledgment email on submit (PRD §3.3.2).
// Implemented by the same jobs.Client adapter the order module uses.
type EmailEnqueuer interface {
	EnqueueEmailSend(ctx context.Context, to, subject, plainText, html string) error
}

// UserFetcher loads the user profile (email + nickname for the ack email +
// auto-attached username). Implemented by user.Service.GetUserProfile.
//
// Widened for the Planner CRM sub-track: ListStaffByRole powers the
// planner-assignment dropdown (PRD §3.3.2). user.Service satisfies both.
type UserDirectory interface {
	GetUserProfile(ctx context.Context, userID string) (*models.User, error)
	ListStaffByRole(ctx context.Context, roleKey string) ([]models.User, error)
}

// ConsentRecorder records the GDPR Privacy Policy consent at submit (PRD §3.3.2
// step 4). Implemented by consent.Service (narrow interface to avoid an
// itinerary→consent import cycle).
type ConsentRecorder interface {
	RecordConsent(ctx context.Context, userID string, kind models.ConsentKind) error
}

// CheckoutFX converts CNY→presentment + returns the raw rate for the quote
// snapshot (TDD §7). Implemented by fx.Service. Mirrors the order module's
// interface verbatim so the same fxService instance satisfies both.
type CheckoutFX interface {
	Convert(ctx context.Context, cnyMinor int64, currency string) (int64, error)
	Rate(ctx context.Context, currency string) (decimal.Decimal, error)
}

// QuotePaymentIntenter creates a gateway intent for an itinerary deposit +
// records the pending payment row (itinerary_quote_id, order_id NULL).
// Implemented by payment.Service.CreateQuoteIntent (injected post-construction
// to break the itinerary↔payment import cycle, mirroring order's setters).
type QuotePaymentIntenter interface {
	CreateQuoteIntent(ctx context.Context, gatewayName string, quoteID int64, amountMinor int64, currency string) (string, error)
}

// QuoteRefunder issues a full refund for a paid quote's succeeded payment via
// the gateway, then marks the payment refunded. Fail-closed: the gateway call
// happens first; a gateway error leaves the quote paid. Implemented by
// payment.Service.RefundQuote.
type QuoteRefunder interface {
	RefundQuote(ctx context.Context, quoteID int64, reason string) error
}

// DepositFinalizeEnqueuer enqueues the itinerary deposit-finalize job (mock
// seam in dev: checkout/pay-deposit auto-succeeds). Implemented by jobs.Client
// via the same adapter the order module uses. Live mode: the gateway webhook
// enqueues it.
type DepositFinalizeEnqueuer interface {
	EnqueueItineraryDepositFinalize(ctx context.Context, quoteID int64, success bool, gateway, gatewayRef string) error
}

// PDFEnqueuer enqueues a pdf:generate job for a freshly sent itinerary quote
// (TDD §12, M3 #4). Narrow interface so the itinerary module doesn't import
// the jobs package. Implemented by the same jobs.Client adapter as the
// certificate module. nil → no PDF job enqueued (worker-side service).
type PDFEnqueuer interface {
	EnqueuePDFGenerate(ctx context.Context, kind string, entityID int64, locale string) error
}

// ServiceInterface defines itinerary business logic (PRD §3.3.2).
type ServiceInterface interface {
	// --- Draft ---
	GetDraft(ctx context.Context, userID string) (*models.ItineraryDraft, error)
	SaveDraft(ctx context.Context, userID string, data models.ItineraryDraftData) (*models.ItineraryDraft, error)
	DeleteDraft(ctx context.Context, userID string) error

	// --- Requests (customer-facing) ---
	Submit(ctx context.Context, userID string, req models.ItinerarySubmitRequest, locale string) (*models.ItineraryRequest, error)
	ListMine(ctx context.Context, userID string, page, limit int) ([]models.ItineraryRequest, int, error)
	GetMine(ctx context.Context, userID string, id int64) (*models.ItineraryRequest, error)
	Cancel(ctx context.Context, userID string, id int64, req models.ItineraryCancelRequest) error

	// --- Planner CRM (PRD §3.3.2 "Backend/CRM") ---
	ListAdmin(ctx context.Context, status, assignedTo, sla string, page, limit int) ([]models.ItineraryAdminRow, int, error)
	GetAdmin(ctx context.Context, id int64) (*models.ItineraryAdminRow, error)
	ListPlanners(ctx context.Context) ([]models.User, error)
	Open(ctx context.Context, id int64) error                  // pending → processing
	Close(ctx context.Context, id int64, req models.ItineraryReasonRequest) error // {pending,processing} → closed
	CancelByStaff(ctx context.Context, id int64, req models.ItineraryReasonRequest) error // {pending,processing} → cancelled
	Assign(ctx context.Context, id int64, req models.AssignItineraryRequest) error
	AddNote(ctx context.Context, requestID int64, authorID string, req models.ItineraryNoteRequest) (*models.CRMNote, error)
	ListNotes(ctx context.Context, requestID int64) ([]models.CRMNote, error)

	// --- Option-rate CMS (PRD §3.3.2: operator-configured rate table) ---
	ListOptionRates(ctx context.Context) ([]models.OptionRate, error)
	CreateOptionRate(ctx context.Context, req models.CreateOptionRateRequest) (*models.OptionRate, error)
	UpdateOptionRate(ctx context.Context, id int64, req models.UpdateOptionRateRequest) (*models.OptionRate, error)
	DeleteOptionRate(ctx context.Context, id int64) error

	// --- Quote builder + deposit (PRD §3.3.2, TDD §3.4 M3 #3) ---
	SendQuote(ctx context.Context, requestID int64, req models.SendQuoteRequest) (*models.ItineraryQuote, error)
	GetQuote(ctx context.Context, requestID int64) (*models.ItineraryQuote, error)
	PayDeposit(ctx context.Context, userID string, requestID int64, req models.PayDepositRequest) (*models.DepositPaidResponse, error)
	MarkDepositPaid(ctx context.Context, quoteID int64) error
	Confirm(ctx context.Context, requestID int64) error
	RefundDeposit(ctx context.Context, requestID int64, req models.RefundDepositRequest) error
}

type Service struct {
	repo      RepositoryInterface
	email     EmailEnqueuer
	user      UserDirectory
	consent   ConsentRecorder
	fx        CheckoutFX // required for quote FX conversion
	// Quote-payment deps, injected post-construction (break the itinerary↔payment
	// import cycle, mirroring order's SetPaymentIntenter/SetPaymentRefunder).
	quoteIntenter QuotePaymentIntenter
	quoteRefunder QuoteRefunder
	depositEnqueuer DepositFinalizeEnqueuer // mock-mode auto-finalize seam
	pdf          PDFEnqueuer             // optional; nil → no PDF job (worker)
	paymentsMode  string                      // "mock" (dev) | "sandbox"/"live" (#6)
}

func NewService(repo RepositoryInterface, email EmailEnqueuer, user UserDirectory, consent ConsentRecorder, fx CheckoutFX, paymentsMode string) *Service {
	return &Service{repo: repo, email: email, user: user, consent: consent, fx: fx, paymentsMode: paymentsMode}
}

// SetQuotePaymentIntenter wires the gateway-intent client post-construction
// (called in main.go after both itinerary + payment services are built).
func (s *Service) SetQuotePaymentIntenter(pi QuotePaymentIntenter) { s.quoteIntenter = pi }

// SetQuoteRefunder wires the gateway-refund client post-construction.
func (s *Service) SetQuoteRefunder(pr QuoteRefunder) { s.quoteRefunder = pr }

// SetDepositFinalizeEnqueuer wires the mock-mode auto-finalize enqueuer
// (serve mode enqueues the job; the worker handles it). Called in main.go.
func (s *Service) SetDepositFinalizeEnqueuer(e DepositFinalizeEnqueuer) { s.depositEnqueuer = e }

// SetPDFEnqueuer wires the pdf:generate enqueuer post-construction (TDD §12).
// Called in main.go; the worker-side service has no enqueuer (pdf stays NULL).
func (s *Service) SetPDFEnqueuer(e PDFEnqueuer) { s.pdf = e }

// --- Draft ---

func (s *Service) GetDraft(ctx context.Context, userID string) (*models.ItineraryDraft, error) {
	return s.repo.GetDraft(ctx, userID)
}

func (s *Service) SaveDraft(ctx context.Context, userID string, data models.ItineraryDraftData) (*models.ItineraryDraft, error) {
	if data.Step < 1 || data.Step > 4 {
		return nil, models.ErrInvalidOperation
	}
	return s.repo.UpsertDraft(ctx, userID, data)
}

func (s *Service) DeleteDraft(ctx context.Context, userID string) error {
	return s.repo.DeleteDraft(ctx, userID)
}

// --- Requests ---

// Submit creates an itinerary request from the 4-step wizard body (PRD §3.3.2):
//  1. Validate the GDPR consent checkbox (hard gate — PRD §3.3.2 step 4).
//  2. Normalize the locale + parse the form fields into the snapshot shape.
//  3. Compute sla_deadline = now + 24h.
//  4. CreateRequest (tx: insert request + delete draft).
//  5. Best-effort: record the consent (audit trail) + enqueue the ack email.
func (s *Service) Submit(ctx context.Context, userID string, req models.ItinerarySubmitRequest, locale string) (*models.ItineraryRequest, error) {
	// 1. Consent gate.
	if !req.Consent {
		return nil, models.ErrConsentRequired
	}

	// 2. Locale + form fields.
	loc, err := i18ncontent.NormalizeLocale(locale, true)
	if err != nil || loc == "" {
		loc = models.DefaultLocale // the service picks a locale even if the client omitted it
	}

	// Parse the arrival date (YYYY-MM-DD).
	var arrival *time.Time
	if req.ArrivalDate != "" {
		t, err := time.Parse("2006-01-02", req.ArrivalDate)
		if err != nil {
			return nil, fmt.Errorf("%w: arrival_date must be YYYY-MM-DD", models.ErrInvalidOperation)
		}
		// Store as a date at midnight UTC (DATE column; the time portion is dropped).
		day := time.Date(t.Year(), t.Month(), t.Day(), 0, 0, 0, 0, time.UTC)
		arrival = &day
	}

	interests, _ := json.Marshal(req.Interests)
	if len(interests) == 0 {
		interests = []byte("[]")
	}
	var budget json.RawMessage
	if req.Budget != nil {
		budget, _ = json.Marshal(req.Budget)
	}
	var services json.RawMessage
	if req.Services != nil {
		services, _ = json.Marshal(req.Services)
	} else {
		services = []byte("{}")
	}
	var contact json.RawMessage
	if req.Contact != nil {
		contact, _ = json.Marshal(req.Contact)
	} else {
		contact = []byte("{}")
	}
	var notes *string
	if req.Notes != "" {
		n := req.Notes
		notes = &n
	}

	// 3. SLA deadline.
	sla := time.Now().Add(slaDuration)

	// 4. Create (tx: insert + delete draft).
	out, err := s.repo.CreateRequest(ctx, &models.ItineraryRequest{
		UserID: userID, ArrivalDate: arrival, DurationDays: req.DurationDays,
		Flexible: req.Flexible, Adults: req.Adults, Children: req.Children,
		Interests: interests, Budget: budget, Pace: req.Pace,
		Services: services, Contact: contact, Notes: notes,
		Locale: loc, SLADeadline: sla,
	})
	if err != nil {
		return nil, err
	}

	// 5. Best-effort: consent audit trail + ack email. A failure is logged +
	// skipped (the request is created; these are side effects, not the
	// transaction). Matches the order module's email posture.
	if s.consent != nil {
		if err := s.consent.RecordConsent(ctx, userID, models.ConsentKindPrivacyPolicy); err != nil {
			log.Printf("itinerary.Submit.Consent(user=%s req=%d): %v", userID, out.ID, err)
		}
	}
	if s.email != nil {
		emailTo := ""
		if s.user != nil {
			if u, err := s.user.GetUserProfile(ctx, userID); err == nil {
				emailTo = u.Email
			}
		}
		if emailTo == "" {
			emailTo = "customer@example.com"
		}
		// PRD §3.3.2: ack email states the 24-hour response SLA.
		subject := "We received your itinerary request"
		plain := "Thank you for your custom itinerary request (#" + strconv.FormatInt(out.ID, 10) +
			"). One of our travel planners will respond within 24 hours."
		if err := s.email.EnqueueEmailSend(ctx, emailTo, subject, plain, ""); err != nil {
			log.Printf("itinerary.Submit.Email(req=%d): %v", out.ID, err)
		}
	}
	return out, nil
}

func (s *Service) ListMine(ctx context.Context, userID string, page, limit int) ([]models.ItineraryRequest, int, error) {
	return s.repo.ListForUser(ctx, userID, page, limit)
}

func (s *Service) GetMine(ctx context.Context, userID string, id int64) (*models.ItineraryRequest, error) {
	return s.repo.GetByIDForUser(ctx, userID, id)
}

func (s *Service) Cancel(ctx context.Context, userID string, id int64, req models.ItineraryCancelRequest) error {
	return s.repo.CancelRequest(ctx, userID, id, req.Reason)
}

// compile-time: keep errors imported (used via models.Err* but errors.Is in
// future cancel-state expansion).
var _ = errors.Is

// =============================================================================
// Planner CRM service methods (PRD §3.3.2 "Backend/CRM")
// =============================================================================

func (s *Service) ListAdmin(ctx context.Context, status, assignedTo, sla string, page, limit int) ([]models.ItineraryAdminRow, int, error) {
	return s.repo.ListAdmin(ctx, status, assignedTo, sla, page, limit)
}

func (s *Service) GetAdmin(ctx context.Context, id int64) (*models.ItineraryAdminRow, error) {
	return s.repo.GetByIDAdmin(ctx, id)
}

// ListPlanners returns active travel_planner users for the assignment dropdown
// (PRD §3.3.2 "assignment of requests to planners").
func (s *Service) ListPlanners(ctx context.Context) ([]models.User, error) {
	if s.user == nil {
		return []models.User{}, nil
	}
	return s.user.ListStaffByRole(ctx, models.RoleTravelPlanner)
}

// Open moves a request pending→processing (a planner is now working it).
func (s *Service) Open(ctx context.Context, id int64) error {
	return s.repo.TransitionStatus(ctx, id, models.StatusItineraryPending, models.StatusItineraryProcessing)
}

// Close moves {pending,processing}→closed. Closed is terminal for the planner
// inbox (the request leaves the active queue).
func (s *Service) Close(ctx context.Context, id int64, req models.ItineraryReasonRequest) error {
	// Try pending→closed first; on conflict (not pending) try processing→closed.
	if err := s.repo.TransitionStatus(ctx, id, models.StatusItineraryPending, models.StatusItineraryClosed); err != nil {
		if errors.Is(err, models.ErrConflict) {
			return s.repo.TransitionStatus(ctx, id, models.StatusItineraryProcessing, models.StatusItineraryClosed)
		}
		return err
	}
	return nil
}

// CancelByStaff moves {pending,processing}→cancelled (planner-initiated). The
// customer's cancel path only allows pending; staff may cancel an in-flight
// (processing) request too, recording the reason for the audit trail.
func (s *Service) CancelByStaff(ctx context.Context, id int64, req models.ItineraryReasonRequest) error {
	return s.repo.CancelByStaff(ctx, id, req.Reason)
}

// Assign sets assigned_to. A non-nil assignee must be an active travel_planner
// (validated against ListPlanners) — guards against stale/invalid UUIDs.
func (s *Service) Assign(ctx context.Context, id int64, req models.AssignItineraryRequest) error {
	if req.AssigneeID != nil && *req.AssigneeID != "" {
		planners, err := s.ListPlanners(ctx)
		if err != nil {
			return fmt.Errorf("itinerary.Assign.ListPlanners: %w", err)
		}
		ok := false
		for _, p := range planners {
			if p.ID == *req.AssigneeID {
				ok = true
				break
			}
		}
		if !ok {
			return fmt.Errorf("%w: assignee is not an active travel planner", models.ErrInvalidOperation)
		}
	}
	return s.repo.Assign(ctx, id, req.AssigneeID)
}

func (s *Service) AddNote(ctx context.Context, requestID int64, authorID string, req models.ItineraryNoteRequest) (*models.CRMNote, error) {
	return s.repo.AddNote(ctx, requestID, authorID, req.Body)
}

func (s *Service) ListNotes(ctx context.Context, requestID int64) ([]models.CRMNote, error) {
	return s.repo.ListNotes(ctx, requestID)
}

// =============================================================================
// Quote builder + deposit service methods (PRD §3.3.2, TDD §3.4 M3 #3)
// =============================================================================

// depositFraction is the PRD §3.3.2 deposit fraction (30%). The balance is due
// 14 days before arrival (collected out-of-band in this sub-track; the full-
// amount pay_full toggle is the MVP alternative).
const depositFraction = 0.30

// roundDeposit rounds total_minor × 0.30 (half-up) to a whole minor unit.
func roundDeposit(totalMinor int64) int64 {
	// totalMinor × 0.30 → (totalMinor × 30 + 50) / 100 (half-up on minor units).
	return (totalMinor*30 + 50) / 100
}

func (s *Service) ListOptionRates(ctx context.Context) ([]models.OptionRate, error) {
	return s.repo.ListOptionRates(ctx)
}

// optionKeyPattern is the canonical option_key format (matches the DB CHECK in
// 000024): lowercase kebab/alphanumeric, starting alphanumeric. Validated in
// the service for a clear 400 before hitting SQL.
var optionKeyPattern = regexp.MustCompile(`^[a-z0-9][a-z0-9_-]*$`)

// CreateOptionRate inserts a new rate row (PRD §3.3.2: operator-configured).
// Validates the option_key regex (the DB CHECK is the backstop). option_key is
// immutable after create.
func (s *Service) CreateOptionRate(ctx context.Context, req models.CreateOptionRateRequest) (*models.OptionRate, error) {
	if !optionKeyPattern.MatchString(req.OptionKey) {
		return nil, fmt.Errorf("%w: option_key must match ^[a-z0-9][a-z0-9_-]*$", models.ErrInvalidOperation)
	}
	return s.repo.CreateOptionRate(ctx, req)
}

// UpdateOptionRate mutates rate_cny/unit/display_label (option_key is immutable).
func (s *Service) UpdateOptionRate(ctx context.Context, id int64, req models.UpdateOptionRateRequest) (*models.OptionRate, error) {
	o, err := s.repo.UpdateOptionRate(ctx, id, req)
	if err != nil {
		if errors.Is(err, models.ErrNotFound) {
			return nil, models.ErrNotFound
		}
		return nil, err
	}
	return o, nil
}

// DeleteOptionRate removes a rate row.
func (s *Service) DeleteOptionRate(ctx context.Context, id int64) error {
	if err := s.repo.DeleteOptionRate(ctx, id); err != nil {
		if errors.Is(err, models.ErrNotFound) {
			return models.ErrNotFound
		}
		return err
	}
	return nil
}

// SendQuote prices the planner's line-item selection from option_rates (CNY),
// converts to the presentment currency via FX (snapshotting fx_rate_used like
// orders), computes the deposit (30% of total, or full if pay_full), and
// creates the quote (replacing any prior quote for the request). The request
// moves processing→quoted. The qty multiplier depends on the option's unit
// (per_person × group size, per_day × duration_days, flat × 1).
func (s *Service) SendQuote(ctx context.Context, requestID int64, req models.SendQuoteRequest) (*models.ItineraryQuote, error) {
	// Load the request (must be processing; pricing needs adults/children/duration).
	r, err := s.repo.GetByIDAdmin(ctx, requestID)
	if err != nil {
		return nil, err
	}
	// A quote can be sent from `processing` (first quote) or `quoted` (a re-quote
	// after the planner iterates). Any other state is invalid.
	if r.Status != models.StatusItineraryProcessing && r.Status != models.StatusItineraryQuoted {
		return nil, models.ErrInvalidOperation
	}
	if s.fx == nil {
		return nil, fmt.Errorf("itinerary.SendQuote: FX service not configured")
	}

	// Resolve option_rates for the selected keys (validate + snapshot rates).
	keys := make([]string, len(req.LineItems))
	for i, li := range req.LineItems {
		keys[i] = li.OptionKey
	}
	rates, err := s.repo.GetOptionRatesByKeys(ctx, keys)
	if err != nil {
		return nil, err
	}
	if len(rates) != len(keys) {
		return nil, fmt.Errorf("%w: unknown option_key in selection", models.ErrInvalidQuote)
	}

	// Price each line (CNY). The planner supplies Qty directly (the multiplier
	// count for the option's unit); per_person/per_day labels are informational.
	lineItems := make([]models.QuoteLineItem, len(req.LineItems))
	var totalCNY int64
	for i, li := range req.LineItems {
		rate, ok := rates[li.OptionKey]
		if !ok {
			return nil, fmt.Errorf("%w: option_key %q not found", models.ErrInvalidQuote, li.OptionKey)
		}
		qty := li.Qty
		if qty < 1 {
			qty = 1
		}
		lineCNY := rate.RateCNY * int64(qty)
		lineItems[i] = models.QuoteLineItem{
			OptionKey: rate.OptionKey, Qty: qty, RateCNY: rate.RateCNY,
			Unit: rate.Unit, Label: rate.DisplayLabel, LineCNY: lineCNY,
		}
		totalCNY += lineCNY
	}
	lineJSON, _ := json.Marshal(lineItems)
	if len(lineJSON) == 0 {
		lineJSON = []byte("[]")
	}

	// FX: convert the CNY total to presentment + snapshot the rate.
	totalMinor, err := s.fx.Convert(ctx, totalCNY, req.Currency)
	if err != nil {
		return nil, fmt.Errorf("itinerary.SendQuote.Convert: %w", err)
	}
	rate, err := s.fx.Rate(ctx, req.Currency)
	if err != nil {
		return nil, fmt.Errorf("itinerary.SendQuote.Rate: %w", err)
	}
	rateStr := rate.StringFixed(8)

	// Deposit: 30% of total (rounded half-up on minor units), or full amount.
	depositMinor := roundDeposit(totalMinor)
	if req.PayFull {
		depositMinor = totalMinor
	}

	// Create the quote (replaces any prior quote, UNIQUE request_id).
	q := &models.ItineraryQuote{
		RequestID: requestID, LineItems: lineJSON, TotalCNY: totalCNY,
		Currency: req.Currency, TotalMinor: totalMinor, DepositMinor: depositMinor,
		FxRateUsed: &rateStr,
	}
	out, err := s.repo.CreateQuote(ctx, q)
	if err != nil {
		return nil, err
	}

	// Move the request processing→quoted (or quoted→quoted is a no-op CAS; for
	// a re-quote the request is already quoted, so the transition is skipped).
	if r.Status == models.StatusItineraryProcessing {
		if err := s.repo.TransitionStatus(ctx, requestID, models.StatusItineraryProcessing, models.StatusItineraryQuoted); err != nil {
			return nil, fmt.Errorf("itinerary.SendQuote.Transition: %w", err)
		}
	}

	// Best-effort: enqueue the itinerary-quote PDF render (TDD §12, M3 #4). The
	// worker renders via chromedp + stores via the storage adapter, populating
	// pdf_key. A failure is logged, never blocks the quote — the quote exists
	// immediately; the PDF can be regenerated later. The render snapshots the
	// quote + request at send time (a re-quote clears pdf_key in CreateQuote's
	// ON CONFLICT, so the new render overwrites the stale PDF).
	if s.pdf != nil {
		if err := s.pdf.EnqueuePDFGenerate(ctx, "itinerary_quote", out.ID, r.Locale); err != nil {
			log.Printf("itinerary.SendQuote.EnqueuePDF(quote=%d): %v", out.ID, err)
		}
	}

	// Best-effort email to the customer (plain text; the PDF is delivered via
	// the download endpoint + the pdf:generate job above).
	if s.email != nil && s.user != nil {
		if u, err := s.user.GetUserProfile(ctx, r.UserID); err == nil {
			subj := "Your itinerary quote is ready (#" + strconv.FormatInt(requestID, 10) + ")"
			body := "A travel planner has prepared a quote for your itinerary request. " +
				"Total: " + req.Currency + " " + strconv.FormatInt(totalMinor/100, 10) + "." +
				"; deposit (30%): " + strconv.FormatInt(depositMinor/100, 10) + ". " +
				"Sign in to review and pay your deposit."
			if err := s.email.EnqueueEmailSend(ctx, u.Email, subj, body, ""); err != nil {
				log.Printf("itinerary.SendQuote.Email(req=%d): %v", requestID, err)
			}
		}
	}
	return out, nil
}

func (s *Service) GetQuote(ctx context.Context, requestID int64) (*models.ItineraryQuote, error) {
	return s.repo.GetQuoteByRequestID(ctx, requestID)
}

// PayDeposit lets the customer pay the deposit (or full amount) for a quoted
// request. Verifies ownership + status==quoted + quote status==sent, then
// creates a gateway intent. mock mode: auto-succeeds (the worker finalizes).
func (s *Service) PayDeposit(ctx context.Context, userID string, requestID int64, req models.PayDepositRequest) (*models.DepositPaidResponse, error) {
	// Ownership + status (must be the customer's own, quoted request).
	r, err := s.repo.GetByIDForUser(ctx, userID, requestID)
	if err != nil {
		return nil, err // ErrNotFound if not the owner
	}
	if r.Status != models.StatusItineraryQuoted {
		return nil, models.ErrRequestNotQuoted
	}
	q, err := s.repo.GetQuoteByRequestID(ctx, requestID)
	if err != nil {
		return nil, err
	}
	if q.Status != models.QuoteSent {
		return nil, models.ErrQuoteAlreadyPaid
	}
	if s.quoteIntenter == nil {
		return nil, models.ErrGatewayUnavailable
	}

	// Amount = deposit_minor (or total_minor if the quote was sent pay_full —
	// the quote's deposit_minor already encodes that at send time).
	amount := q.DepositMinor

	hosted, err := s.quoteIntenter.CreateQuoteIntent(ctx, req.Gateway, q.ID, amount, q.Currency)
	if err != nil {
		return nil, err
	}

	// mock mode: auto-enqueue the finalize job (dev seam, mirrors checkout).
	if s.paymentsMode == "mock" && s.depositEnqueuer != nil {
		gatewayRef := "mock-" + strconv.FormatInt(q.ID, 10)
		if err := s.depositEnqueuer.EnqueueItineraryDepositFinalize(ctx, q.ID, true, "mock", gatewayRef); err != nil {
			log.Printf("itinerary.PayDeposit.Enqueue(quote=%d): %v (manual finalize needed)", q.ID, err)
		}
	}

	return &models.DepositPaidResponse{QuoteID: q.ID, HostedURL: hosted}, nil
}

// MarkDepositPaid is the worker-side finalize: CAS quote sent→deposit_paid
// (idempotent on replay) + move the request quoted→deposit_paid. Called by the
// worker's payment:finalize handler on a succeeded deposit webhook.
func (s *Service) MarkDepositPaid(ctx context.Context, quoteID int64) error {
	q, err := s.repo.GetQuoteByID(ctx, quoteID)
	if err != nil {
		return err // ErrNotFound → webhook acks (no quote to finalize)
	}
	payFull := q.Status == models.QuoteFullyPaid || (q.DepositMinor == q.TotalMinor)
	won, err := s.repo.MarkQuoteDepositPaid(ctx, quoteID, payFull)
	if err != nil {
		return err
	}
	if !won {
		return nil // already paid (replayed webhook) — idempotent
	}
	// Move the request quoted→deposit_paid. On a CAS miss (concurrent cancel),
	// the quote is deposit_paid but the request stays quoted — the planner
	// reconciles. Non-fatal; log + return nil so the webhook acks 200.
	if err := s.repo.TransitionStatus(ctx, q.RequestID, models.StatusItineraryQuoted, models.StatusItineraryDepositPaid); err != nil {
		log.Printf("itinerary.MarkDepositPaid.Transition(quote=%d req=%d): %v", quoteID, q.RequestID, err)
	}
	return nil
}

// Confirm moves a deposit_paid request → confirmed (planner action; PDF +
// email deferred to the chromedp sub-track).
func (s *Service) Confirm(ctx context.Context, requestID int64) error {
	return s.repo.TransitionStatus(ctx, requestID, models.StatusItineraryDepositPaid, models.StatusItineraryConfirmed)
}

// RefundDeposit issues a full refund of the paid deposit (fail-closed: gateway
// first, then quote + request cancelled). PRD §3.3.2 tiered partial refunds are
// a deferred follow-up (need partial-refund gateway support + a policy engine).
func (s *Service) RefundDeposit(ctx context.Context, requestID int64, req models.RefundDepositRequest) error {
	q, err := s.repo.GetQuoteByRequestID(ctx, requestID)
	if err != nil {
		return err
	}
	if q.Status != models.QuoteDepositPaid && q.Status != models.QuoteFullyPaid {
		return models.ErrConflict
	}
	// Fail-closed: gateway refund first.
	if s.quoteRefunder != nil {
		if err := s.quoteRefunder.RefundQuote(ctx, q.ID, req.Reason); err != nil {
			return err
		}
	}
	if err := s.repo.CancelQuote(ctx, q.ID); err != nil {
		return err
	}
	// Move the request →cancelled (from deposit_paid). Reuse CancelByStaff's
	// CAS which accepts {pending,processing}; deposit_paid needs its own.
	if err := s.repo.TransitionStatus(ctx, requestID, models.StatusItineraryDepositPaid, models.StatusItineraryCancelled); err != nil {
		// If not deposit_paid (e.g. already cancelled), fall back to a direct
		// cancelled set — but only after the gateway refund succeeded.
		log.Printf("itinerary.RefundDeposit.Transition(req=%d): %v", requestID, err)
	}
	return nil
}
