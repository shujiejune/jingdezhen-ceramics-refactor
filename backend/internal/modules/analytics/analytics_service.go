package analytics

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"time"

	"jingdezhen-ceramics-backend/internal/models"
	"jingdezhen-ceramics-backend/pkg/adapters/geoip"
)

// ConsentChecker is the narrow seam the analytics service uses to enforce the
// cookie_analytics consent gate (PRD §4.3: "analytics blocked until consent").
// Implemented by consent.Service — injected so analytics doesn't import consent.
type ConsentChecker interface {
	GetConsentStateForIP(ctx context.Context, clientIP string, kind models.ConsentKind) (*models.ConsentRecord, error)
}

// ServiceInterface defines analytics business logic.
type ServiceInterface interface {
	// Record validates, gates on consent, geo-resolves, hashes the visitor,
	// and stores one event. Returns models.ErrConsentNotGranted (→ handler 204)
	// when the visitor has not granted cookie_analytics consent; the event is
	// NOT stored in that case.
	Record(ctx context.Context, clientIP, userAgent string, req models.AnalyticsEventRequest) (*models.AnalyticsEvent, error)
	// RollupDaily aggregates the given day's events into analytics_daily. Pass
	// time.Time{} for "yesterday" (the analytics:rollup nightly job). Idempotent.
	RollupDaily(ctx context.Context, day time.Time) error
}

type Service struct {
	repo    RepositoryInterface
	consent ConsentChecker
	geoip   geoip.Lookup
	hmacKey []byte // app key for the daily-rotating visitor_hash
}

func NewService(repo RepositoryInterface, consent ConsentChecker, geoipLookup geoip.Lookup, hmacKey []byte) ServiceInterface {
	return &Service{repo: repo, consent: consent, geoip: geoipLookup, hmacKey: hmacKey}
}

func (s *Service) Record(ctx context.Context, clientIP, userAgent string, req models.AnalyticsEventRequest) (*models.AnalyticsEvent, error) {
	// --- Consent gate (PRD §4.3) ---
	rec, err := s.consent.GetConsentStateForIP(ctx, clientIP, models.ConsentKindCookieAnalytics)
	if err != nil {
		return nil, fmt.Errorf("analytics.Record: consent check: %w", err)
	}
	if rec == nil || !rec.Granted {
		// Silently drop: not a client error. Handler returns 204 No Content.
		return nil, models.ErrConsentNotGranted
	}

	// --- Geo-resolve at ingest (TDD §196) ---
	country, _ := s.geoip.Country(clientIP) // ZZ on miss/noop

	var namePtr *string
	if req.Name != "" {
		n := req.Name
		namePtr = &n
	}
	var localePtr *string
	if req.Locale != "" {
		l := req.Locale
		localePtr = &l
	}
	if req.Props == nil {
		req.Props = map[string]any{}
	}

	ev := models.AnalyticsEvent{
		Ts:          time.Now(),
		Kind:        req.Kind,
		Path:        req.Path,
		Name:        namePtr,
		Country:     country,
		Locale:      localePtr,
		VisitorHash: s.visitorHash(clientIP, userAgent),
		Props:       req.Props,
	}
	if err := s.repo.Insert(ctx, ev); err != nil {
		return nil, fmt.Errorf("analytics.Record: %w", err)
	}
	return &ev, nil
}

func (s *Service) RollupDaily(ctx context.Context, day time.Time) error {
	if day.IsZero() {
		day = time.Now().UTC().AddDate(0, 0, -1) // yesterday (UTC)
	}
	date := day.UTC().Format("2006-01-02")
	if err := s.repo.RollupPageviews(ctx, date); err != nil {
		return fmt.Errorf("analytics.RollupDaily: %w", err)
	}
	if err := s.repo.RollupEvents(ctx, date); err != nil {
		return fmt.Errorf("analytics.RollupDaily: %w", err)
	}
	if err := s.repo.RollupVisitors(ctx, date); err != nil {
		return fmt.Errorf("analytics.RollupDaily: %w", err)
	}
	return nil
}

// visitorHash returns hex(HMAC(dailyKey, IP+"\n"+UA)). The daily key is
// HMAC(appKey, YYYY-MM-DD), derived deterministically (no storage) so the
// same visitor collides within a day (correct unique-visitor counts) but
// the hash changes across days — a DB leak cannot cross-day track a person
// (GDPR-friendly, TDD §11).
func (s *Service) visitorHash(ip, ua string) string {
	day := time.Now().UTC().Format("20060102")
	dailyKey := hmacSHA256(string(s.hmacKey), day)
	return hex.EncodeToString(hmacSHA256(string(dailyKey), ip+"\n"+ua))
}

func hmacSHA256(key, msg string) []byte {
	mac := hmac.New(sha256.New, []byte(key))
	mac.Write([]byte(msg))
	return mac.Sum(nil)
}

var _ = errors.Is // reserved for future sentinel wrapping without an import churn
