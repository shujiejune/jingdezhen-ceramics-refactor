package privacy

import (
	"context"
	"errors"
	"fmt"
	"log"
	"time"

	"jingdezhen-ceramics-backend/internal/models"
	"jingdezhen-ceramics-backend/internal/platform/i18ncontent"
	"jingdezhen-ceramics-backend/internal/platform/jobs"
	"jingdezhen-ceramics-backend/pkg/adapters/tokenblocklist"
)

// EmailEnqueuer is the subset of the Asynq job client the privacy service needs
// (mirrors the user-service interface) so deletion confirmations are sent via
// the email:send queue, never inline (TDD §4.2).
type EmailEnqueuer interface {
	EnqueueEmailSend(ctx context.Context, p jobs.EmailSendPayload) error
}

// TokenRevoker invalidates a user's outstanding access tokens. The privacy
// service calls it on erasure so a deleted user's existing JWT stops working
// immediately (TDD §5.1 stopgap) instead of lingering up to the token TTL.
type TokenRevoker interface {
	Revoke(ctx context.Context, userID string, ttl time.Duration) error
}

// ServiceInterface defines GDPR self-service operations (PRD §4.3).
type ServiceInterface interface {
	// ExportUserData assembles the user's personal-data package (synchronous for
	// MVP — a single user's data volume is small; switched to an async job +
	// download-link email once order/itinerary history lands in M2/M3).
	ExportUserData(ctx context.Context, userID string, locale string) (*models.UserDataExport, error)

	// DeleteAccount performs irreversible GDPR erasure: anonymizes the user row
	// (PII nulled, is_active=false, deleted_at set), CASCADE-purges addresses /
	// 2FA / favorites / notifications, and enqueues a confirmation email to the
	// user's (pre-erasure) address. Returns models.ErrAccountDeleted if already
	// erased.
	DeleteAccount(ctx context.Context, userID string) error
}

type Service struct {
	repo          RepositoryInterface
	emailEnqueuer EmailEnqueuer
	tokenRevoker  TokenRevoker
}

func NewService(repo RepositoryInterface, emailEnqueuer EmailEnqueuer, tokenRevoker TokenRevoker) ServiceInterface {
	return &Service{repo: repo, emailEnqueuer: emailEnqueuer, tokenRevoker: tokenRevoker}
}

func (s *Service) ExportUserData(ctx context.Context, userID string, locale string) (*models.UserDataExport, error) {
	// Normalize the locale for the export's metadata; an invalid locale doesn't
	// block the export (the user may have an empty preferred_locale).
	norm, _ := i18ncontent.NormalizeLocale(locale, false)
	exp, err := s.repo.ExportUserData(ctx, userID, norm)
	if err != nil {
		return nil, fmt.Errorf("service.ExportUserData: %w", err)
	}
	return exp, nil
}

func (s *Service) DeleteAccount(ctx context.Context, userID string) error {
	// Capture the profile before erasure so we can email the (soon-to-be-removed)
	// address with a deletion confirmation. If the profile lookup fails we still
	// proceed with erasure — the email is best-effort, not a gate.
	export, err := s.repo.ExportUserData(ctx, userID, "")
	if err != nil {
		if errors.Is(err, models.ErrNotFound) {
			return models.ErrNotFound
		}
		// A transient DB error here is a real failure — don't erase blindly.
		return fmt.Errorf("service.DeleteAccount.prefetch: %w", err)
	}

	if err := s.repo.AnonymizeUser(ctx, userID); err != nil {
		return err
	}

	// Revoke the user's outstanding access tokens so they stop working
	// immediately (TDD §5.1 stopgap). Best-effort: a Redis outage here only
	// means the deleted user's token lingers up to its own expiry; login is
	// already blocked by is_active=false. Never blocks the erasure.
	if s.tokenRevoker != nil {
		if err := s.tokenRevoker.Revoke(ctx, userID, tokenblocklist.MaxAccessTokenTTL); err != nil {
			log.Printf("privacy.DeleteAccount: token revoke failed for %s: %v", userID, err)
		}
	}

	// Best-effort deletion confirmation email. The user's email was just nulled
	// in the DB, but we captured it above. Enqueuing (not sending inline) means
	// a Brevo outage never blocks the erasure or leaves it half-done.
	if export.Profile != nil && export.Profile.Email != "" {
		_ = s.emailEnqueuer.EnqueueEmailSend(ctx, jobs.EmailSendPayload{
			To:        export.Profile.Email,
			Subject:   "[Jingdezhen Ceramics] Your account has been deleted",
			PlainText: "Your Jingdezhen Ceramics account and all associated personal data have been permanently deleted in line with your GDPR erasure request. This action cannot be undone. If you did not initiate this request, please contact our support team. — Jingdezhen Ceramics Platform",
			HTML:      "<p>Your Jingdezhen Ceramics account and all associated personal data have been permanently deleted in line with your GDPR erasure request.</p><p><strong>This action cannot be undone.</strong></p><p>If you did not initiate this request, please contact our support team.</p><p>— Jingdezhen Ceramics Platform</p>",
		})
	}
	return nil
}
