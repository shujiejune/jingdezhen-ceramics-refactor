package audit

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"log"

	"jingdezhen-ceramics-backend/internal/models"
)

// Logger is the seam other services use to record a sensitive action (PRD
// §3.1.1). It is injected (nullable) so tests + the worker don't need it; a
// nil Logger means the call is a no-op (NoopLogger). The actor IP is hashed
// here (HMAC, reusing CONSENT_HMAC_KEY) — no raw IP is persisted.
type Logger interface {
	Log(ctx context.Context, actorID, actorIP string, action models.AuditAction,
		entityType models.AuditEntityType, entityID string, detail map[string]any) error
}

// ServiceInterface defines audit business logic.
type ServiceInterface interface {
	Logger
	// List returns audit rows matching filter, newest first, paginated.
	List(ctx context.Context, f models.AuditLogFilter, page, limit int) ([]models.AuditLog, int, error)
}

type Service struct {
	repo    RepositoryInterface
	pool    Executor // the pool, for the no-tx best-effort path
	hmacKey []byte   // CONSENT_HMAC_KEY reused (same short-term audit purpose)
}

func NewService(repo RepositoryInterface, hmacKey []byte, pool Executor) ServiceInterface {
	return &Service{repo: repo, hmacKey: hmacKey, pool: pool}
}

// Log records one audit entry. `exec` should be the caller's transaction (for
// atomicity) or nil (the service uses the pool via repo — but Insert needs an
// Executor, so callers without a tx pass nil and we... actually the repo needs
// an Executor; we wrap the pool. For the no-tx path the caller passes nil and
// we log via a best-effort separate write — NOT atomic with the action, but
// acceptable for actions that don't already hold a tx). Most sensitive paths
// already hold a tx or are single statements where the audit row's atomicity
// is less critical.
//
// Best-effort: a failure is logged + returned but callers MUST NOT block the
// action on it (the action already succeeded). The pattern is:
//
//	if err := s.audit.Log(...); err != nil { log.Printf("audit: %v", err) }
func (s *Service) Log(ctx context.Context, actorID, actorIP string, action models.AuditAction,
	entityType models.AuditEntityType, entityID string, detail map[string]any) error {

	entry := models.AuditLog{
		Action:     action,
		EntityType: entityType,
		Detail:     detail,
	}
	if actorID != "" {
		a := actorID
		entry.ActorID = &a
	}
	if actorIP != "" {
		h := hashIP(s.hmacKey, actorIP)
		entry.ActorIPHash = &h
	}
	if entityID != "" {
		e := entityID
		entry.EntityID = &e
	}

	// No-tx path: the service's pool. Callers that want atomicity should call
	// repo.Insert directly with their tx (the Logger interface is the simple
	// path; the tx path is an internal optimisation for the few services that
	// need it). For MVP the no-tx path is fine — the action already succeeded.
	if err := s.repo.Insert(ctx, s.pool, entry); err != nil {
		log.Printf("audit.Log: %v (action=%s entity=%s/%s)", err, action, entityType, entityID)
		return err
	}
	return nil
}

func (s *Service) List(ctx context.Context, f models.AuditLogFilter, page, limit int) ([]models.AuditLog, int, error) {
	rows, total, err := s.repo.List(ctx, f, page, limit)
	if err != nil {
		return nil, 0, fmt.Errorf("audit.List: %w", err)
	}
	return rows, total, nil
}

// NoopLogger is a Logger that does nothing. Used in tests + the worker where
// audit recording isn't wired.
type NoopLogger struct{}

func (NoopLogger) Log(context.Context, string, string, models.AuditAction,
	models.AuditEntityType, string, map[string]any) error {
	return nil
}

// hashIP returns hex(HMAC-SHA256(key, ip)). Mirrors consent.hashIP — the same
// short-term audit/dedup purpose, not re-identification (GDPR minimisation).
func hashIP(key []byte, ip string) string {
	mac := hmac.New(sha256.New, key)
	mac.Write([]byte(ip))
	return hex.EncodeToString(mac.Sum(nil))
}
