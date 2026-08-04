package itinerary

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"strconv"
	"time"

	"jingdezhen-ceramics-backend/internal/models"
	"jingdezhen-ceramics-backend/internal/platform/i18ncontent"
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
}

type Service struct {
	repo      RepositoryInterface
	email     EmailEnqueuer
	user      UserDirectory
	consent   ConsentRecorder
}

func NewService(repo RepositoryInterface, email EmailEnqueuer, user UserDirectory, consent ConsentRecorder) *Service {
	return &Service{repo: repo, email: email, user: user, consent: consent}
}

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
