package analytics_test

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"jingdezhen-ceramics-backend/internal/models"
	"jingdezhen-ceramics-backend/internal/modules/analytics"
	"jingdezhen-ceramics-backend/internal/modules/consent"
	"jingdezhen-ceramics-backend/internal/testutil"
	"jingdezhen-ceramics-backend/pkg/adapters/geoip"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const consentKey = "test-consent-key"
const analyticsKey = "test-analytics-key"

// recordConsentForIP inserts a cookie_analytics consent row by IP hash, mirroring
// consent.Service.RecordConsent but without needing an HTTP layer.
func recordConsentForIP(t *testing.T, db *pgxpool.Pool, ip string, granted bool) {
	t.Helper()
	repo := consent.NewRepository(db)
	svc := consent.NewService(repo, []byte(consentKey))
	_, err := svc.RecordConsent(context.Background(), nil, ip, models.RecordConsentRequest{
		Kind:       models.ConsentKindCookieAnalytics,
		DocVersion: "1.0",
		Granted:    granted,
	})
	require.NoError(t, err)
}

func newSvc(db *pgxpool.Pool, geoipLookup geoip.Lookup) analytics.ServiceInterface {
	consentRepo := consent.NewRepository(db)
	consentSvc := consent.NewService(consentRepo, []byte(consentKey))
	repo := analytics.NewRepository(db)
	return analytics.NewService(repo, consentSvc, geoipLookup, []byte(analyticsKey))
}

func countEvents(t *testing.T, db *pgxpool.Pool) int {
	t.Helper()
	var n int
	err := db.QueryRow(context.Background(), `SELECT COUNT(*) FROM analytics_events`).Scan(&n)
	require.NoError(t, err)
	return n
}

// TestRecord_InsertsRowWhenConsented verifies the happy path: a consented visitor's
// event lands in analytics_events with country 'ZZ' (noop geoip), a populated
// visitor_hash, and props JSONB round-tripping.
func TestRecord_InsertsRowWhenConsented(t *testing.T) {
	db := testutil.NewDBPool(t)
	ctx := context.Background()

	const ip = "1.2.3.4"
	recordConsentForIP(t, db, ip, true)

	svc := newSvc(db, geoip.NewNoop())
	_, err := svc.Record(ctx, ip, "UA-test", models.AnalyticsEventRequest{
		Kind:   models.AnalyticsKindEvent,
		Path:   "/custom-travel",
		Name:   "itinerary_form_view",
		Locale: "en-US",
		Props:  map[string]any{"step": 1},
	})
	require.NoError(t, err)
	assert.Equal(t, 1, countEvents(t, db))

	// Read back: country ZZ, visitor_hash populated, props round-trips.
	var country, vhash, locale string
	var name *string
	var props []byte
	err = db.QueryRow(ctx, `SELECT country, visitor_hash, locale, name, props FROM analytics_events WHERE path = '/custom-travel'`).Scan(&country, &vhash, &locale, &name, &props)
	require.NoError(t, err)
	assert.Equal(t, "ZZ", country)
	assert.Equal(t, "en-US", locale)
	assert.NotEmpty(t, vhash)
	require.NotNil(t, name)
	assert.Equal(t, "itinerary_form_view", *name)

	var p map[string]any
	require.NoError(t, json.Unmarshal(props, &p))
	assert.Equal(t, float64(1), p["step"])
}

// TestRecord_DropsWhenConsentNotGranted: not-consented → ErrConsentNotGranted +
// zero rows in analytics_events.
func TestRecord_DropsWhenConsentNotGranted(t *testing.T) {
	db := testutil.NewDBPool(t)
	ctx := context.Background()

	const ip = "5.6.7.8"
	recordConsentForIP(t, db, ip, false) // explicitly refused

	svc := newSvc(db, geoip.NewNoop())
	_, err := svc.Record(ctx, ip, "UA", models.AnalyticsEventRequest{
		Kind: models.AnalyticsKindPageview, Path: "/",
	})
	require.ErrorIs(t, err, models.ErrConsentNotGranted)
	assert.Equal(t, 0, countEvents(t, db), "no event stored when consent refused")
}

// TestRecord_DropsWhenNoConsentRecord: never-consented visitor → dropped.
func TestRecord_DropsWhenNoConsentRecord(t *testing.T) {
	db := testutil.NewDBPool(t)
	ctx := context.Background()

	svc := newSvc(db, geoip.NewNoop())
	_, err := svc.Record(ctx, "9.9.9.9", "UA", models.AnalyticsEventRequest{
		Kind: models.AnalyticsKindPageview, Path: "/",
	})
	require.ErrorIs(t, err, models.ErrConsentNotGranted)
	assert.Equal(t, 0, countEvents(t, db))
}

// TestRecord_GeoipResolved is covered by TestGeoIPMaxMind_GBCountry below, which
// uses the GeoIP package's own testdata fixture path.

// TestRollup_AggregatesAndIdempotent: seed a day's events, rollup, verify
// analytics_daily, then rollup again and verify values unchanged.
func TestRollup_AggregatesAndIdempotent(t *testing.T) {
	db := testutil.NewDBPool(t)
	ctx := context.Background()
	repo := analytics.NewRepository(db)

	day := time.Date(2026, 8, 5, 12, 0, 0, 0, time.UTC)
	dayStr := "2026-08-05"

	// Seed 3 pageviews (path A, US, en) + 2 events (named, path A, US, en) from
	// 2 distinct visitor hashes so visitors=2.
	seed := func(kind models.AnalyticsEventKind, path string, name *string, vhash string) {
		nb, _ := json.Marshal(name)
		var nameArg any
		if name != nil {
			nameArg = *name
		}
		_, err := db.Exec(ctx, `
			INSERT INTO analytics_events (ts, kind, path, name, country, locale, visitor_hash, props)
			VALUES ($1, $2, $3, $4, 'US', 'en-US', $5, '{}'::jsonb)`,
			day, kind, path, nameArg, vhash)
		require.NoError(t, err)
		_ = nb
	}
	pv := models.AnalyticsKindPageview
	ev := models.AnalyticsKindEvent
	seed(pv, "/products", nil, "vA")
	seed(pv, "/products", nil, "vA") // same visitor, 2nd pageview
	seed(pv, "/products", nil, "vB") // 2nd visitor
	skuName := "add_to_cart"
	seed(ev, "/products", &skuName, "vA")
	seed(ev, "/products", &skuName, "vB")

	svc := analytics.NewService(repo, nil, geoip.NewNoop(), []byte(analyticsKey))
	require.NoError(t, svc.RollupDaily(ctx, day))

	// Expect daily rows: pageviews{path=/products,country=US,locale=en-US}=3,
	// events{name=add_to_cart,...}=2, visitors{country=US,locale=en-US}=2.
	var (
		pageviews, events, visitors int64
	)
	err := db.QueryRow(ctx, `
		SELECT value FROM analytics_daily
		WHERE date=$1 AND metric='pageviews' AND dims @> '{"path":"/products","country":"US","locale":"en-US"}'::jsonb`,
		dayStr).Scan(&pageviews)
	require.NoError(t, err)
	assert.Equal(t, int64(3), pageviews)

	err = db.QueryRow(ctx, `
		SELECT value FROM analytics_daily
		WHERE date=$1 AND metric='events' AND dims @> '{"name":"add_to_cart","country":"US","locale":"en-US"}'::jsonb`,
		dayStr).Scan(&events)
	require.NoError(t, err)
	assert.Equal(t, int64(2), events)

	err = db.QueryRow(ctx, `
		SELECT value FROM analytics_daily
		WHERE date=$1 AND metric='visitors' AND dims @> '{"country":"US","locale":"en-US"}'::jsonb`,
		dayStr).Scan(&visitors)
	require.NoError(t, err)
	assert.Equal(t, int64(2), visitors, "2 distinct visitor_hash values")

	// Idempotency: re-run the rollup for the same day; values must not double.
	require.NoError(t, svc.RollupDaily(ctx, day))
	require.NoError(t, db.QueryRow(ctx, `
		SELECT value FROM analytics_daily
		WHERE date=$1 AND metric='pageviews' AND dims @> '{"path":"/products"}'::jsonb`,
		dayStr).Scan(&pageviews))
	assert.Equal(t, int64(3), pageviews, "re-run rollup is idempotent (set, not increment)")

	// DailyCount sanity (repo helper used by the no-op branch).
	repo2 := analytics.NewRepository(db)
	n, err := repo2.DailyCount(ctx, dayStr)
	require.NoError(t, err)
	assert.Greater(t, n, 0)
}

// TestRollup_EmptyDayIsNoOp: rolling up a day with zero events inserts zero rows.
func TestRollup_EmptyDayIsNoOp(t *testing.T) {
	db := testutil.NewDBPool(t)
	ctx := context.Background()
	repo := analytics.NewRepository(db)
	svc := analytics.NewService(repo, nil, geoip.NewNoop(), []byte(analyticsKey))

	day := time.Date(2026, 8, 6, 0, 0, 0, 0, time.UTC)
	require.NoError(t, svc.RollupDaily(ctx, day))
	n, err := repo.DailyCount(ctx, "2026-08-06")
	require.NoError(t, err)
	assert.Equal(t, 0, n, "empty day rollup writes no rows")
}

// TestGeoIPMaxMind_GBCountry verifies the analytics service wires a real
// MaxMind lookup end-to-end (using the package's own testdata fixture path).
func TestGeoIPMaxMind_GBCountry(t *testing.T) {
	db := testutil.NewDBPool(t)
	ctx := context.Background()

	const ip = "81.2.69.160"
	recordConsentForIP(t, db, ip, true)

	mm, err := geoip.NewTestMaxMind()
	require.NoError(t, err)
	t.Cleanup(func() { _ = mm.Close() })

	repo := analytics.NewRepository(db)
	consentSvc := consent.NewService(consent.NewRepository(db), []byte(consentKey))
	svc := analytics.NewService(repo, consentSvc, mm, []byte(analyticsKey))

	_, err = svc.Record(ctx, ip, "UA", models.AnalyticsEventRequest{
		Kind: models.AnalyticsKindPageview, Path: "/gb-test",
	})
	require.NoError(t, err)

	var country string
	require.NoError(t, db.QueryRow(ctx, `SELECT country FROM analytics_events WHERE path='/gb-test'`).Scan(&country))
	assert.Equal(t, "GB", country, "GeoIP resolves the test fixture IP to GB")
}
