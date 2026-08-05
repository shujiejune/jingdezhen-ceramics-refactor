package analytics

import (
	"context"
	"testing"
	"time"

	"jingdezhen-ceramics-backend/internal/models"
	"jingdezhen-ceramics-backend/pkg/adapters/geoip"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// stubConsent implements ConsentChecker for tests.
type stubConsent struct {
	rec   *models.ConsentRecord
	err   error
	calls int
}

func (s *stubConsent) GetConsentStateForIP(_ context.Context, _ string, _ models.ConsentKind) (*models.ConsentRecord, error) {
	s.calls++
	return s.rec, s.err
}

func (s *stubConsent) granted(granted bool) *stubConsent {
	s.rec = &models.ConsentRecord{Granted: granted}
	return s
}

// stubRepo records the last inserted event for assertions.
type stubRepo struct {
	inserted  *models.AnalyticsEvent
	insertErr error
	rollups   int
}

func (r *stubRepo) Insert(_ context.Context, e models.AnalyticsEvent) error {
	if r.insertErr != nil {
		return r.insertErr
	}
	r.inserted = &e
	return nil
}
func (r *stubRepo) RollupPageviews(context.Context, string) error   { r.rollups++; return nil }
func (r *stubRepo) RollupEvents(context.Context, string) error      { r.rollups++; return nil }
func (r *stubRepo) RollupVisitors(context.Context, string) error    { r.rollups++; return nil }
func (r *stubRepo) DailyCount(context.Context, string) (int, error) { return 0, nil }

func newSvc(t *testing.T) (*Service, *stubRepo, *stubConsent) {
	t.Helper()
	repo := &stubRepo{}
	consent := &stubConsent{}
	svc := NewService(repo, consent, geoip.NewNoop(), []byte("test-analytics-key"))
	return svc.(*Service), repo, consent
}

func TestRecord_NoConsent_DropsEvent(t *testing.T) {
	svc, repo, consent := newSvc(t)
	consent.granted(false) // not granted
	_, err := svc.Record(context.Background(), "1.2.3.4", "UA", models.AnalyticsEventRequest{
		Kind: models.AnalyticsKindPageview, Path: "/",
	})
	require.ErrorIs(t, err, models.ErrConsentNotGranted)
	assert.Nil(t, repo.inserted, "no event stored when consent not granted")
	assert.Equal(t, 1, consent.calls, "consent gate called exactly once")
}

func TestRecord_NoConsentRecord_DropsEvent(t *testing.T) {
	// nil record (never consented) == not granted.
	svc, repo, consent := newSvc(t)
	consent.rec = nil
	_, err := svc.Record(context.Background(), "1.2.3.4", "UA", models.AnalyticsEventRequest{
		Kind: models.AnalyticsKindPageview, Path: "/",
	})
	require.ErrorIs(t, err, models.ErrConsentNotGranted)
	assert.Nil(t, repo.inserted)
}

func TestRecord_Granted_StoresWithZZAndHash(t *testing.T) {
	svc, repo, consent := newSvc(t)
	consent.granted(true)
	name := "itinerary_form_view"
	ev, err := svc.Record(context.Background(), "1.2.3.4", "UA-test", models.AnalyticsEventRequest{
		Kind:   models.AnalyticsKindEvent,
		Path:   "/custom-travel",
		Name:   name,
		Locale: "en-US",
		Props:  map[string]any{"step": 1},
	})
	require.NoError(t, err)
	require.NotNil(t, ev)
	assert.Equal(t, "ZZ", repo.inserted.Country, "noop geoip → ZZ")
	assert.Equal(t, "ZZ", ev.Country)
	require.NotNil(t, repo.inserted.Name)
	assert.Equal(t, name, *repo.inserted.Name)
	require.NotNil(t, repo.inserted.Locale)
	assert.Equal(t, "en-US", *repo.inserted.Locale)
	assert.Equal(t, ev.VisitorHash, repo.inserted.VisitorHash)
	assert.NotEmpty(t, ev.VisitorHash)
	assert.Equal(t, map[string]any{"step": 1}, repo.inserted.Props)
}

func TestRecord_PageviewHasNilName(t *testing.T) {
	svc, repo, consent := newSvc(t)
	consent.granted(true)
	_, err := svc.Record(context.Background(), "1.2.3.4", "UA", models.AnalyticsEventRequest{
		Kind: models.AnalyticsKindPageview, Path: "/products",
	})
	require.NoError(t, err)
	assert.Nil(t, repo.inserted.Name, "pageview → name NULL")
}

func TestVisitorHash_StableWithinDay_DifferentUA(t *testing.T) {
	// Same IP+UA on the same day → identical hash (same visitor).
	svc, repo, consent := newSvc(t)
	consent.granted(true)
	_, _ = svc.Record(context.Background(), "1.2.3.4", "UA", models.AnalyticsEventRequest{Kind: models.AnalyticsKindPageview, Path: "/"})
	h1 := repo.inserted.VisitorHash
	_, _ = svc.Record(context.Background(), "1.2.3.4", "UA", models.AnalyticsEventRequest{Kind: models.AnalyticsKindPageview, Path: "/"})
	h2 := repo.inserted.VisitorHash
	assert.Equal(t, h1, h2, "same IP+UA same day collides")

	// Different UA → different hash (different visitor).
	_, _ = svc.Record(context.Background(), "1.2.3.4", "DIFFERENT-UA", models.AnalyticsEventRequest{Kind: models.AnalyticsKindPageview, Path: "/"})
	h3 := repo.inserted.VisitorHash
	assert.NotEqual(t, h1, h3, "different UA → different hash")
}

func TestVisitorHash_KeyDependsOnAppKey(t *testing.T) {
	// Two services with different app keys must produce different hashes for the
	// same IP+UA (a key rotation invalidates prior visitor identities — by design).
	repo1, repo2 := &stubRepo{}, &stubRepo{}
	consent := &stubConsent{}
	consent.granted(true)
	svc1 := NewService(repo1, consent, geoip.NewNoop(), []byte("key-A")).(*Service)
	svc2 := NewService(repo2, consent, geoip.NewNoop(), []byte("key-B")).(*Service)
	assert.NotEqual(t,
		svc1.visitorHash("1.2.3.4", "UA"),
		svc2.visitorHash("1.2.3.4", "UA"),
		"different app keys → different visitor hashes")
}

func TestRollupDaily_YesterdayWhenZero(t *testing.T) {
	svc, repo, _ := newSvc(t)
	// time.Time{} → yesterday UTC.
	err := svc.RollupDaily(context.Background(), time.Time{})
	require.NoError(t, err)
	assert.Equal(t, 3, repo.rollups, "pageviews + events + visitors rollups each run once")
}

func TestRollupDaily_ExplicitDate(t *testing.T) {
	svc, repo, _ := newSvc(t)
	err := svc.RollupDaily(context.Background(), time.Date(2026, 8, 5, 0, 0, 0, 0, time.UTC))
	require.NoError(t, err)
	assert.Equal(t, 3, repo.rollups)
}
