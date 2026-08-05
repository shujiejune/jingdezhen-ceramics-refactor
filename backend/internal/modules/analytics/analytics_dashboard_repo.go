package analytics

import (
	"context"
	"fmt"
	"time"

	"jingdezhen-ceramics-backend/internal/models"

	"github.com/jackc/pgx/v5/pgxpool"
)

// maxBreakdown caps the per-breakdown lists (by_country/by_path/...) returned
// by the dashboard reads. MVP volume is low, but a cap keeps the response
// payload bounded + the UI simple.
const maxBreakdown = 100

// DashboardRepo is the storage seam for the dashboard read endpoints (PRD
// §3.4.2 Phase B). It is a separate interface from RepositoryInterface so the
// ingest/rollup repository stays focused; both are satisfied by the same
// *pgxpool.Pool-backed *Repository.
type DashboardRepo interface {
	Traffic(ctx context.Context, from, to time.Time) (*models.TrafficReport, error)
	Sales(ctx context.Context, from, to time.Time) (*models.SalesReport, error)
	Funnel(ctx context.Context, from, to time.Time) (*models.FunnelReport, error)
}

// NewDashboardRepo returns a DashboardRepo backed by db.
func NewDashboardRepo(db *pgxpool.Pool) DashboardRepo { return &Repository{db: db} }

// dateDay returns the UTC YYYY-MM-DD string for a time (used for the Series).
func dateDay(t time.Time) string { return t.UTC().Format("2006-01-02") }

// --- Traffic -----------------------------------------------------------------

// Traffic reads live from analytics_events. The dashboard does NOT read
// analytics_daily (TDD §12): for MVP volume the live query is cheap + always
// correct (no "today not rolled up yet" gap). analytics_daily stays wired for a
// future scale-out switch.
func (r *Repository) Traffic(ctx context.Context, from, to time.Time) (*models.TrafficReport, error) {
	rng := models.TrafficReport{From: from, To: to}

	// Totals: pageviews, distinct visitors, named events.
	const totalsQ = `
		SELECT
			COUNT(*) FILTER (WHERE kind = 'pageview')::bigint,
			COUNT(DISTINCT visitor_hash)::bigint,
			COUNT(*) FILTER (WHERE kind = 'event')::bigint
		FROM analytics_events
		WHERE ts >= $1 AND ts < $2`
	row := r.db.QueryRow(ctx, totalsQ, from, to)
	if err := row.Scan(&rng.Totals.Pageviews, &rng.Totals.Visitors, &rng.Totals.Events); err != nil {
		return nil, fmt.Errorf("repo.Traffic.totals: %w", err)
	}

	// By country (pageviews + distinct visitors).
	const countryQ = `
		SELECT country,
			COUNT(*) FILTER (WHERE kind = 'pageview')::bigint,
			COUNT(DISTINCT visitor_hash)::bigint
		FROM analytics_events
		WHERE ts >= $1 AND ts < $2
		GROUP BY country
		ORDER BY 2 DESC, country
		LIMIT $3`
	if err := scanTrafficByCountry(ctx, r, countryQ, from, to, &rng.ByCountry); err != nil {
		return nil, fmt.Errorf("repo.Traffic.by_country: %w", err)
	}

	// Top content (pageviews by path).
	const pathQ = `
		SELECT path, COUNT(*)::bigint
		FROM analytics_events
		WHERE kind = 'pageview' AND ts >= $1 AND ts < $2
		GROUP BY path ORDER BY 2 DESC, path LIMIT $3`
	if err := scanTrafficByPath(ctx, r, pathQ, from, to, &rng.ByPath); err != nil {
		return nil, fmt.Errorf("repo.Traffic.by_path: %w", err)
	}

	// By locale (pageviews + visitors). NULL locale → 'unknown'.
	const localeQ = `
		SELECT COALESCE(locale, 'unknown'),
			COUNT(*) FILTER (WHERE kind = 'pageview')::bigint,
			COUNT(DISTINCT visitor_hash)::bigint
		FROM analytics_events
		WHERE ts >= $1 AND ts < $2
		GROUP BY 1 ORDER BY 2 DESC, 1 LIMIT $3`
	if err := scanTrafficByLocale(ctx, r, localeQ, from, to, &rng.ByLocale); err != nil {
		return nil, fmt.Errorf("repo.Traffic.by_locale: %w", err)
	}

	// Daily series.
	const seriesQ = `
		SELECT (ts AT TIME ZONE 'UTC')::date AS d,
			COUNT(*) FILTER (WHERE kind = 'pageview')::bigint,
			COUNT(DISTINCT visitor_hash)::bigint,
			COUNT(*) FILTER (WHERE kind = 'event')::bigint
		FROM analytics_events
		WHERE ts >= $1 AND ts < $2
		GROUP BY d ORDER BY d`
	seriesMap, err := scanDailyTraffic(ctx, r, seriesQ, from, to)
	if err != nil {
		return nil, fmt.Errorf("repo.Traffic.series: %w", err)
	}
	rng.Series = zeroFillTraffic(from, to, seriesMap)
	return &rng, nil
}

// --- Sales -------------------------------------------------------------------

// Sales GMV = SUM(subtotal_cny) (merchandise, excludes shipping) over realized
// orders only (status IN paid|shipped|completed — cancelled/refunded excluded).
const realizedOrderStatus = "('paid','shipped','completed')"

func (r *Repository) Sales(ctx context.Context, from, to time.Time) (*models.SalesReport, error) {
	rng := models.SalesReport{From: from, To: to}

	const totalsQ = `
		SELECT COALESCE(SUM(subtotal_cny),0)::bigint, COUNT(*)::bigint
		FROM orders
		WHERE status IN ` + realizedOrderStatus + ` AND placed_at >= $1 AND placed_at < $2`
	if err := r.db.QueryRow(ctx, totalsQ, from, to).Scan(&rng.Totals.GMVCny, &rng.Totals.Orders); err != nil {
		return nil, fmt.Errorf("repo.Sales.totals: %w", err)
	}

	// By presentment currency.
	const curQ = `
		SELECT currency, COALESCE(SUM(subtotal_cny),0)::bigint, COUNT(*)::bigint
		FROM orders
		WHERE status IN ` + realizedOrderStatus + ` AND placed_at >= $1 AND placed_at < $2
		GROUP BY currency ORDER BY 2 DESC, currency LIMIT $3`
	rows, err := r.db.Query(ctx, curQ, from, to, maxBreakdown)
	if err != nil {
		return nil, fmt.Errorf("repo.Sales.by_currency: %w", err)
	}
	defer rows.Close()
	for rows.Next() {
		var v models.SalesByCurrency
		if err := rows.Scan(&v.Currency, &v.GMVCny, &v.Orders); err != nil {
			return nil, fmt.Errorf("repo.Sales.by_currency.scan: %w", err)
		}
		rng.ByCurrency = append(rng.ByCurrency, v)
	}

	// By shipping-address country (address->>'country'). NULL/missing → 'unknown'.
	const countryQ = `
		SELECT COALESCE(address->>'country','unknown'),
			COALESCE(SUM(subtotal_cny),0)::bigint, COUNT(*)::bigint
		FROM orders
		WHERE status IN ` + realizedOrderStatus + ` AND placed_at >= $1 AND placed_at < $2
		GROUP BY 1 ORDER BY 2 DESC, 1 LIMIT $3`
	rows, err = r.db.Query(ctx, countryQ, from, to, maxBreakdown)
	if err != nil {
		return nil, fmt.Errorf("repo.Sales.by_country: %w", err)
	}
	defer rows.Close()
	for rows.Next() {
		var v models.SalesByCountry
		if err := rows.Scan(&v.Country, &v.GMVCny, &v.Orders); err != nil {
			return nil, fmt.Errorf("repo.Sales.by_country.scan: %w", err)
		}
		rng.ByCountry = append(rng.ByCountry, v)
	}

	// By product (order_items JOIN skus/products + en-US product title fallback).
	const productQ = `
		SELECT p.id,
			COALESCE(pt.title, 'Unknown'),
			COALESCE(SUM(oi.unit_price_cny * oi.qty),0)::bigint,
			COALESCE(SUM(oi.qty),0)::bigint
		FROM order_items oi
		JOIN orders o ON o.id = oi.order_id
		JOIN skus s ON s.id = oi.sku_id
		JOIN products p ON p.id = s.product_id
		LEFT JOIN LATERAL (
			SELECT title FROM product_translations
			WHERE product_id = p.id AND locale = 'en-US'
			ORDER BY published_at DESC NULLS LAST LIMIT 1
		) pt ON true
		WHERE o.status IN ` + realizedOrderStatus + ` AND o.placed_at >= $1 AND o.placed_at < $2
		GROUP BY p.id, pt.title ORDER BY 3 DESC, 2, p.id LIMIT $3`
	rows, err = r.db.Query(ctx, productQ, from, to, maxBreakdown)
	if err != nil {
		return nil, fmt.Errorf("repo.Sales.by_product: %w", err)
	}
	defer rows.Close()
	for rows.Next() {
		var v models.SalesByProduct
		if err := rows.Scan(&v.ProductID, &v.Title, &v.GMVCny, &v.Qty); err != nil {
			return nil, fmt.Errorf("repo.Sales.by_product.scan: %w", err)
		}
		rng.ByProduct = append(rng.ByProduct, v)
	}

	// By artist (products.artist_id; NULL artist_id → 'Unknown').
	const artistQ = `
		SELECT COALESCE(a.id, 0),
			COALESCE(a.name, 'Unknown'),
			COALESCE(SUM(oi.unit_price_cny * oi.qty),0)::bigint,
			COALESCE(SUM(oi.qty),0)::bigint
		FROM order_items oi
		JOIN orders o ON o.id = oi.order_id
		JOIN skus s ON s.id = oi.sku_id
		JOIN products p ON p.id = s.product_id
		LEFT JOIN artists a ON a.id = p.artist_id
		WHERE o.status IN ` + realizedOrderStatus + ` AND o.placed_at >= $1 AND o.placed_at < $2
		GROUP BY a.id, a.name ORDER BY 3 DESC, 2, a.id LIMIT $3`
	rows, err = r.db.Query(ctx, artistQ, from, to, maxBreakdown)
	if err != nil {
		return nil, fmt.Errorf("repo.Sales.by_artist: %w", err)
	}
	defer rows.Close()
	for rows.Next() {
		var v models.SalesByArtist
		if err := rows.Scan(&v.ArtistID, &v.Name, &v.GMVCny, &v.Qty); err != nil {
			return nil, fmt.Errorf("repo.Sales.by_artist.scan: %w", err)
		}
		rng.ByArtist = append(rng.ByArtist, v)
	}

	// Daily series.
	const seriesQ = `
		SELECT (placed_at AT TIME ZONE 'UTC')::date AS d,
			COALESCE(SUM(subtotal_cny),0)::bigint, COUNT(*)::bigint
		FROM orders
		WHERE status IN ` + realizedOrderStatus + ` AND placed_at >= $1 AND placed_at < $2
		GROUP BY d ORDER BY d`
	seriesMap, err := scanDailySales(ctx, r, seriesQ, from, to)
	if err != nil {
		return nil, fmt.Errorf("repo.Sales.series: %w", err)
	}
	rng.Series = zeroFillSales(from, to, seriesMap)
	return &rng, nil
}

// --- Funnel ------------------------------------------------------------------

// FunnelViewEventName is the contract the frontend must fire on the custom-
// travel form load (PRD §3.4.2 "form views"). The dashboard counts analytics
// events with this exact name as the funnel's top stage. An unknown name simply
// yields zero views — it is not validated server-side.
const FunnelViewEventName = "itinerary_form_view"

func (r *Repository) Funnel(ctx context.Context, from, to time.Time) (*models.FunnelReport, error) {
	rng := models.FunnelReport{From: from, To: to}

	// Views: analytics_events named itinerary_form_view.
	const viewsQ = `SELECT COUNT(*)::bigint FROM analytics_events
		WHERE kind = 'event' AND name = $1 AND ts >= $2 AND ts < $3`
	if err := r.db.QueryRow(ctx, viewsQ, FunnelViewEventName, from, to).Scan(&rng.Totals.Views); err != nil {
		return nil, fmt.Errorf("repo.Funnel.views: %w", err)
	}

	// Submitted + confirmed (cohort: submitted_at in range; confirmed = same
	// rows with status='confirmed'). No confirmed_at column — funnel uses cohort
	// semantics (TDD §12) rather than a timestamp.
	const itinQ = `
		SELECT COUNT(*)::bigint,
			COUNT(*) FILTER (WHERE status = 'confirmed')::bigint
		FROM itinerary_requests
		WHERE submitted_at >= $1 AND submitted_at < $2`
	if err := r.db.QueryRow(ctx, itinQ, from, to).Scan(&rng.Totals.Submitted, &rng.Totals.Confirmed); err != nil {
		return nil, fmt.Errorf("repo.Funnel.submitted: %w", err)
	}

	// Daily series: views (analytics_events) + submitted/confirmed (itinerary).
	const viewsSeriesQ = `
		SELECT (ts AT TIME ZONE 'UTC')::date AS d, COUNT(*)::bigint
		FROM analytics_events
		WHERE kind = 'event' AND name = $1 AND ts >= $2 AND ts < $3
		GROUP BY d`
	viewsMap, err := scanDailyCount(ctx, r, viewsSeriesQ, FunnelViewEventName, from, to)
	if err != nil {
		return nil, fmt.Errorf("repo.Funnel.views_series: %w", err)
	}
	const itinSeriesQ = `
		SELECT (submitted_at AT TIME ZONE 'UTC')::date AS d,
			COUNT(*)::bigint,
			COUNT(*) FILTER (WHERE status = 'confirmed')::bigint
		FROM itinerary_requests
		WHERE submitted_at >= $1 AND submitted_at < $2
		GROUP BY d`
	submittedMap, confirmedMap, err := scanDailyFunnelItin(ctx, r, itinSeriesQ, from, to)
	if err != nil {
		return nil, fmt.Errorf("repo.Funnel.itin_series: %w", err)
	}
	rng.Series = zeroFillFunnel(from, to, viewsMap, submittedMap, confirmedMap)
	return &rng, nil
}
