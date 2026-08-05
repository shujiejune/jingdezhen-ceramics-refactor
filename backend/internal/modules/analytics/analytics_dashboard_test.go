package analytics_test

import (
	"context"
	"encoding/json"
	"io"
	"net/http/httptest"
	"testing"
	"time"

	"jingdezhen-ceramics-backend/internal/modules/analytics"
	"jingdezhen-ceramics-backend/internal/testutil"
	"jingdezhen-ceramics-backend/pkg/adapters/geoip"

	"github.com/gofiber/fiber/v2"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// dashDB seeds a deterministic 3-day window (2026-08-01..03 UTC) and returns
// the pool + the inclusive-from/exclusive-to window the dashboard should query.
//
//	analytics_events: 2 pageviews (US/en-US, /products) + 1 event (itinerary_form_view)
//	                  on day 1; 1 pageview (CN/zh-CN, /products) on day 2; nothing day 3.
//	orders: day1 paid order subtotal_cny=1000 (USD, address country US)
//	        day2 shipped order subtotal_cny=2000 (EUR, address country DE) — cancelled
//	        day3 completed order subtotal_cny=4000 (USD, address country US)
//	order_items: day1: sku1×2 @ 500 cny (product P1, artist A1)
//	             day3: sku2×1 @ 4000 cny (product P2, artist NULL)
//	itinerary_requests: day1 submitted + confirmed; day2 submitted (not confirmed).
//
// GMV (realized: paid|shipped|completed) = 1000 (day1 paid) + 4000 (day3
// completed) = 5000. The day2 shipped order is cancelled → excluded.
func dashDB(t *testing.T) (*pgxpool.Pool, time.Time, time.Time) {
	t.Helper()
	db := testutil.NewDBPool(t)
	ctx := context.Background()
	uid := seedCustomerUUID(t, db) // one customer for all orders + itinerary
	exec := func(q string, args ...any) {
		t.Helper()
		_, err := db.Exec(ctx, q, args...)
		require.NoError(t, err)
	}

	// --- seed products, skus, artists (minimal columns) ---
	// One statement per Exec: pgx uses prepared statements, which reject
	// multiple commands in one call.
	exec(`INSERT INTO artists (id, name) VALUES (1, 'Artist One')`)
	exec(`INSERT INTO products (id, artist_id) VALUES (1, 1), (2, NULL)`)
	exec(`INSERT INTO product_translations (product_id, locale, title, slug, status, published_at)
		VALUES (1,'en-US','Vase','vase','published','2026-01-01'), (2,'en-US','Bowl','bowl','published','2026-01-01')`)
	exec(`INSERT INTO skus (id, product_id, sku_code, price_cny, stock, weight_grams, attributes, is_active)
		VALUES (1,1,'SKU1',500,10,500,'{}'::jsonb,true), (2,2,'SKU2',4000,5,800,'{}'::jsonb,true)`)

	// --- analytics_events (day1 + day2) ---
	insertEvent := func(day int, kind, name, country, locale, vhash string) {
		ts := time.Date(2026, 8, day, 12, 0, 0, 0, time.UTC)
		var nameArg any
		if name != "" {
			nameArg = name
		}
		_, err := db.Exec(ctx, `
			INSERT INTO analytics_events (ts, kind, path, name, country, locale, visitor_hash, props)
			VALUES ($1, $2, '/products', $3, $4, $5, $6, '{}'::jsonb)`,
			ts, kind, nameArg, country, locale, vhash)
		require.NoError(t, err)
	}
	insertEvent(1, "pageview", "", "US", "en-US", "v1")
	insertEvent(1, "pageview", "", "US", "en-US", "v2")
	insertEvent(1, "event", "itinerary_form_view", "US", "en-US", "v1")
	insertEvent(2, "pageview", "", "CN", "zh-CN", "v3")

	// --- orders + order_items (one statement per Exec) ---
	day1 := time.Date(2026, 8, 1, 12, 0, 0, 0, time.UTC)
	day2 := time.Date(2026, 8, 2, 12, 0, 0, 0, time.UTC)
	day3 := time.Date(2026, 8, 3, 12, 0, 0, 0, time.UTC)

	// Day1: paid, USD, country US, subtotal_cny=1000, item sku1×2 @500.
	exec(`INSERT INTO orders (id, user_id, status, currency, subtotal_minor, shipping_minor, total_minor,
		                    subtotal_cny, shipping_cny, total_cny, address, placed_at, paid_at)
		VALUES (1, $1, 'paid', 'USD', 1400, 100, 1500, 1000, 70, 1070,
			'{"country":"US"}'::jsonb, $2, $2)`, uid, day1)
	exec(`INSERT INTO order_items (order_id, sku_id, qty, unit_price_minor, unit_price_cny)
		VALUES (1, 1, 2, 700, 500)`)

	// Day2: cancelled (excluded from GMV).
	exec(`INSERT INTO orders (id, user_id, status, currency, subtotal_minor, shipping_minor, total_minor,
		                    subtotal_cny, shipping_cny, total_cny, address, placed_at, cancelled_at)
		VALUES (2, $1, 'cancelled', 'EUR', 2800, 0, 2800, 2000, 0, 2000,
			'{"country":"DE"}'::jsonb, $2, $2)`, uid, day2)

	// Day3: completed, USD, country US, subtotal_cny=4000, item sku2×1 @4000.
	exec(`INSERT INTO orders (id, user_id, status, currency, subtotal_minor, shipping_minor, total_minor,
		                    subtotal_cny, shipping_cny, total_cny, address, placed_at, completed_at)
		VALUES (3, $1, 'completed', 'USD', 5600, 0, 5600, 4000, 0, 4000,
			'{"country":"US"}'::jsonb, $2, $2)`, uid, day3)
	exec(`INSERT INTO order_items (order_id, sku_id, qty, unit_price_minor, unit_price_cny)
		VALUES (3, 2, 1, 5600, 4000)`)

	// --- itinerary_requests (day1 submitted+confirmed, day2 submitted) ---
	exec(`INSERT INTO itinerary_requests (user_id, status, duration_days, adults, pace, interests,
			budget, services, contact, locale, sla_deadline, submitted_at)
		VALUES ($1, 'confirmed', 3, 2, 'balanced', '[]'::jsonb, '{}'::jsonb, '{}'::jsonb,
			'{"channel":"email"}'::jsonb, 'en-US', $2, $2),
			($1, 'pending', 2, 1, 'relaxed', '[]'::jsonb, '{}'::jsonb, '{}'::jsonb,
			'{"channel":"email"}'::jsonb, 'zh-CN', $3, $3)`,
		uid, day1, day2)

	from := time.Date(2026, 8, 1, 0, 0, 0, 0, time.UTC)
	to := time.Date(2026, 8, 4, 0, 0, 0, 0, time.UTC) // exclusive: covers days 1-3
	return db, from, to
}

// seedCustomerUUID creates a minimal customer user row (orders/itinerary FK) +
// returns its UUID. Customers are users WITHOUT a user_roles row (per the RBAC
// design — migration 000002). No `role` column on users.
func seedCustomerUUID(t *testing.T, db *pgxpool.Pool) string {
	t.Helper()
	var id string
	err := db.QueryRow(context.Background(), `
		INSERT INTO users (nickname, email, password_hash, is_active, auth_provider)
		VALUES ('Dash Test', 'dash-test@jingdezhen.test', 'x', true, 'email')
		RETURNING id::text`).Scan(&id)
	require.NoError(t, err)
	return id
}

func newDashService(db *pgxpool.Pool) analytics.DashboardServiceInterface {
	return analytics.NewDashboardService(analytics.NewDashboardRepo(db))
}

func TestDashboard_Traffic(t *testing.T) {
	db, from, to := dashDB(t)
	svc := newDashService(db)
	rep, err := svc.TrafficReport(context.Background(), from, to)
	require.NoError(t, err)

	// Totals: 3 pageviews (2 day1 US + 1 day2 CN), 3 distinct visitors, 1 named event.
	assert.Equal(t, int64(3), rep.Totals.Pageviews)
	assert.Equal(t, int64(3), rep.Totals.Visitors)
	assert.Equal(t, int64(1), rep.Totals.Events)

	// ByCountry: US (2 pageviews, 2 visitors), CN (1,1).
	require.Len(t, rep.ByCountry, 2)
	us := rep.ByCountry[0]
	assert.Equal(t, "US", us.Country)
	assert.Equal(t, int64(2), us.Pageviews)
	assert.Equal(t, int64(2), us.Visitors)
	cn := rep.ByCountry[1]
	assert.Equal(t, "CN", cn.Country)
	assert.Equal(t, int64(1), cn.Pageviews)

	// ByPath: /products with 3 pageviews.
	require.Len(t, rep.ByPath, 1)
	assert.Equal(t, "/products", rep.ByPath[0].Path)
	assert.Equal(t, int64(3), rep.ByPath[0].Pageviews)

	// ByLocale: en-US (2 pageviews, 2 visitors), zh-CN (1,1).
	require.Len(t, rep.ByLocale, 2)
	assert.Equal(t, "en-US", rep.ByLocale[0].Locale)
	assert.Equal(t, int64(2), rep.ByLocale[0].Pageviews)

	// Series: 3 contiguous days, zero-filled.
	require.Len(t, rep.Series, 3)
	assert.Equal(t, "2026-08-01", rep.Series[0].Date)
	assert.Equal(t, int64(2), rep.Series[0].Pageviews)
	assert.Equal(t, int64(1), rep.Series[0].Events)
	assert.Equal(t, "2026-08-02", rep.Series[1].Date)
	assert.Equal(t, int64(1), rep.Series[1].Pageviews)
	assert.Equal(t, "2026-08-03", rep.Series[2].Date)
	assert.Equal(t, int64(0), rep.Series[2].Pageviews, "day3 zero-filled")
}

func TestDashboard_Sales(t *testing.T) {
	db, from, to := dashDB(t)
	svc := newDashService(db)
	rep, err := svc.SalesReport(context.Background(), from, to)
	require.NoError(t, err)

	// GMV = 1000 (day1 paid) + 4000 (day3 completed) = 5000. day2 cancelled excluded.
	assert.Equal(t, int64(5000), rep.Totals.GMVCny)
	assert.Equal(t, int64(2), rep.Totals.Orders, "only paid + completed (cancelled excluded)")

	// ByCurrency: USD (1000+4000=5000, 2 orders).
	require.Len(t, rep.ByCurrency, 1)
	assert.Equal(t, "USD", rep.ByCurrency[0].Currency)
	assert.Equal(t, int64(5000), rep.ByCurrency[0].GMVCny)
	assert.Equal(t, int64(2), rep.ByCurrency[0].Orders)

	// ByCountry: US (5000, 2). (DE cancelled → excluded.)
	require.Len(t, rep.ByCountry, 1)
	assert.Equal(t, "US", rep.ByCountry[0].Country)
	assert.Equal(t, int64(5000), rep.ByCountry[0].GMVCny)

	// ByProduct: P1 (Vase) GMV=1000 qty=2; P2 (Bowl) GMV=4000 qty=1.
	require.Len(t, rep.ByProduct, 2)
	assert.Equal(t, "Bowl", rep.ByProduct[0].Title, "sorted by GMV desc → P2 first")
	assert.Equal(t, int64(4000), rep.ByProduct[0].GMVCny)
	assert.Equal(t, int64(1), rep.ByProduct[0].Qty)
	assert.Equal(t, "Vase", rep.ByProduct[1].Title)
	assert.Equal(t, int64(1000), rep.ByProduct[1].GMVCny)
	assert.Equal(t, int64(2), rep.ByProduct[1].Qty)

	// ByArtist: A1 (Artist One) GMV=1000 qty=2; Unknown (NULL artist) GMV=4000 qty=1.
	require.Len(t, rep.ByArtist, 2)
	assert.Equal(t, "Unknown", rep.ByArtist[0].Name, "NULL artist_id → Unknown, top by GMV")
	assert.Equal(t, int64(4000), rep.ByArtist[0].GMVCny)
	assert.Equal(t, "Artist One", rep.ByArtist[1].Name)
	assert.Equal(t, int64(1000), rep.ByArtist[1].GMVCny)

	// Series: 3 days. day1 GMV=1000 orders=1; day2 0/0 (cancelled); day3 4000/1.
	require.Len(t, rep.Series, 3)
	assert.Equal(t, int64(1000), rep.Series[0].GMVCny)
	assert.Equal(t, int64(1), rep.Series[0].Orders)
	assert.Equal(t, int64(0), rep.Series[1].GMVCny, "day2 cancelled → zero GMV")
	assert.Equal(t, int64(4000), rep.Series[2].GMVCny)
}

func TestDashboard_Funnel(t *testing.T) {
	db, from, to := dashDB(t)
	svc := newDashService(db)
	rep, err := svc.FunnelReport(context.Background(), from, to)
	require.NoError(t, err)

	// Views: 1 (day1 itinerary_form_view). Submitted: 2 (day1 + day2). Confirmed: 1 (day1).
	assert.Equal(t, int64(1), rep.Totals.Views)
	assert.Equal(t, int64(2), rep.Totals.Submitted)
	assert.Equal(t, int64(1), rep.Totals.Confirmed)

	// Conversion: view→submit = 2/1 = 200%; submit→confirm = 1/2 = 50%.
	assert.Equal(t, 200.0, rep.Conversion.ViewToSubmit)
	assert.Equal(t, 50.0, rep.Conversion.SubmitToConfirm)

	// Series: 3 days. day1 views=1 submitted=1 confirmed=1; day2 0/1/0; day3 0/0/0.
	require.Len(t, rep.Series, 3)
	assert.Equal(t, "2026-08-01", rep.Series[0].Date)
	assert.Equal(t, int64(1), rep.Series[0].Views)
	assert.Equal(t, int64(1), rep.Series[0].Submitted)
	assert.Equal(t, int64(1), rep.Series[0].Confirmed)
	assert.Equal(t, "2026-08-02", rep.Series[1].Date)
	assert.Equal(t, int64(0), rep.Series[1].Views)
	assert.Equal(t, int64(1), rep.Series[1].Submitted)
	assert.Equal(t, "2026-08-03", rep.Series[2].Date)
	assert.Equal(t, int64(0), rep.Series[2].Submitted, "day3 zero-filled")
}

func TestDashboard_Funnel_ZeroDenominators(t *testing.T) {
	// A range with no data → 0 conversions (no NaN).
	db := testutil.NewDBPool(t)
	svc := newDashService(db)
	rep, err := svc.FunnelReport(context.Background(),
		time.Date(2026, 8, 10, 0, 0, 0, 0, time.UTC),
		time.Date(2026, 8, 11, 0, 0, 0, 0, time.UTC))
	require.NoError(t, err)
	assert.Equal(t, 0.0, rep.Conversion.ViewToSubmit)
	assert.Equal(t, 0.0, rep.Conversion.SubmitToConfirm)
}

// TestDashboard_CSV verifies the ?format=csv path streams text/csv with the
// expected header + rows.
func TestDashboard_CSV(t *testing.T) {
	db, _, _ := dashDB(t)
	svc := newDashService(db)
	h := analytics.NewDashboardHandler(svc)

	app := fiber.New()
	app.Get("/sales", func(c *fiber.Ctx) error {
		c.Request().URI().SetQueryString("from=2026-08-01&to=2026-08-03&format=csv")
		return h.Sales(c)
	})

	req := httptest.NewRequest("GET", "/sales", nil)
	resp, err := app.Test(req)
	require.NoError(t, err)
	assert.Equal(t, "text/csv", resp.Header.Get("Content-Type"))
	assert.Contains(t, resp.Header.Get("Content-Disposition"), "analytics-sales.csv")
	body, _ := io.ReadAll(resp.Body)
	assert.Contains(t, string(body), "date,gmv_cny,orders")
	assert.Contains(t, string(body), "2026-08-01,1000,1")
	assert.Contains(t, string(body), "2026-08-03,4000,1")
	assert.NotContains(t, string(body), "2026-08-02,2000", "cancelled day excluded from GMV")
}

// sanity: the seeded funnel view event name matches the constant.
func TestDashboard_FunnelEventNameConstant(t *testing.T) {
	assert.Equal(t, "itinerary_form_view", analytics.FunnelViewEventName)
}

// _ keeps the geoip import alive for future geo-resolved dashboard tests.
var _ = geoip.NewNoop

// _ keeps the json import alive (used by other dashboard tests that assert JSON).
var _ = json.Marshal
