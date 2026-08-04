package consent

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"log"
	"time"

	"jingdezhen-ceramics-backend/internal/models"
)

// ServiceInterface defines consent-ledger business logic.
type ServiceInterface interface {
	// RecordConsent appends a consent record. `requesterUserID` is the
	// authenticated user (nil for an anonymous visitor); `clientIP` is hashed
	// (HMAC, daily-rotating key) before storage — no raw IPs are persisted.
	RecordConsent(ctx context.Context, requesterUserID *string, clientIP string, req models.RecordConsentRequest) (*models.ConsentRecord, error)
	// GetConsentState returns the latest granted/refused record for a user +
	// kind (nil = no record = not yet consented).
	GetConsentState(ctx context.Context, userID string, kind models.ConsentKind) (*models.ConsentRecord, error)
	// ListUserConsentHistory returns the full consent history for a user (GDPR
	// data export). Caller is responsible for ensuring the requester is allowed
	// to view this user's data (own data, or admin).
	ListUserConsentHistory(ctx context.Context, userID string) ([]models.ConsentRecord, error)
}

type Service struct {
	repo    RepositoryInterface
	hmacKey []byte // secret for IP hashing; rotate daily in prod (TODO: key rotation)
}

// NewService constructs the consent service. `hmacKey` seeds the IP hash HMAC;
// in production it should rotate daily (TDD §11). For MVP a static key from env
// is acceptable — the hash's purpose is short-term dedup/audit, not
// re-identification.
func NewService(repo RepositoryInterface, hmacKey []byte) ServiceInterface {
	return &Service{repo: repo, hmacKey: hmacKey}
}

func (s *Service) RecordConsent(ctx context.Context, requesterUserID *string, clientIP string, req models.RecordConsentRequest) (*models.ConsentRecord, error) {
	rec := models.ConsentRecord{
		UserID:     requesterUserID,
		Kind:       req.Kind,
		DocVersion: req.DocVersion,
		Granted:    req.Granted,
		CreatedAt:  time.Now(),
	}
	if clientIP != "" {
		h := hashIP(s.hmacKey, clientIP)
		rec.IPHash = &h
	}

	out, err := s.repo.Record(ctx, rec)
	if err != nil {
		return nil, fmt.Errorf("service.RecordConsent: %w", err)
	}
	log.Printf("Consent recorded: user=%v kind=%s granted=%t doc=%s",
		requesterUserID, req.Kind, req.Granted, req.DocVersion)
	return out, nil
}

func (s *Service) GetConsentState(ctx context.Context, userID string, kind models.ConsentKind) (*models.ConsentRecord, error) {
	rec, err := s.repo.LatestForUser(ctx, userID, kind)
	if err != nil {
		if errors.Is(err, models.ErrNotFound) {
			return nil, nil // no record = not yet consented (not an error for the caller)
		}
		return nil, fmt.Errorf("service.GetConsentState: %w", err)
	}
	return rec, nil
}

func (s *Service) ListUserConsentHistory(ctx context.Context, userID string) ([]models.ConsentRecord, error) {
	history, err := s.repo.ListForUser(ctx, userID)
	if err != nil {
		return nil, fmt.Errorf("service.ListUserConsentHistory: %w", err)
	}
	return history, nil
}

// hashIP returns the HMAC-SHA256 hex digest of an IP address. The key rotates
// daily in production so the hash is only useful for short-term dedup/audit,
// not long-term re-identification (GDPR minimisation, TDD §11).
func hashIP(key []byte, ip string) string {
	mac := hmac.New(sha256.New, key)
	mac.Write([]byte(ip))
	return hex.EncodeToString(mac.Sum(nil))
}
