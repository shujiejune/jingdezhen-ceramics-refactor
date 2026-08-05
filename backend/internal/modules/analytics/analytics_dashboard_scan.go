package analytics

import (
	"context"
	"time"

	"jingdezhen-ceramics-backend/internal/models"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// --- scan helpers (dashboard reads) ------------------------------------------
// These keep the dashboard repo file free of repetitive row-scan blocks. All
// return maps keyed by UTC YYYY-MM-DD for the zero-fill pass.

func scanTrafficByCountry(ctx context.Context, r *Repository, q string, from, to time.Time, out *[]models.TrafficByCountry) error {
	rows, err := r.db.Query(ctx, q, from, to, maxBreakdown)
	if err != nil {
		return err
	}
	defer rows.Close()
	for rows.Next() {
		var v models.TrafficByCountry
		if err := rows.Scan(&v.Country, &v.Pageviews, &v.Visitors); err != nil {
			return err
		}
		*out = append(*out, v)
	}
	return rows.Err()
}

func scanTrafficByPath(ctx context.Context, r *Repository, q string, from, to time.Time, out *[]models.TrafficByPath) error {
	rows, err := r.db.Query(ctx, q, from, to, maxBreakdown)
	if err != nil {
		return err
	}
	defer rows.Close()
	for rows.Next() {
		var v models.TrafficByPath
		if err := rows.Scan(&v.Path, &v.Pageviews); err != nil {
			return err
		}
		*out = append(*out, v)
	}
	return rows.Err()
}

func scanTrafficByLocale(ctx context.Context, r *Repository, q string, from, to time.Time, out *[]models.TrafficByLocale) error {
	rows, err := r.db.Query(ctx, q, from, to, maxBreakdown)
	if err != nil {
		return err
	}
	defer rows.Close()
	for rows.Next() {
		var v models.TrafficByLocale
		if err := rows.Scan(&v.Locale, &v.Pageviews, &v.Visitors); err != nil {
			return err
		}
		*out = append(*out, v)
	}
	return rows.Err()
}

// scanDailyTraffic returns map[date]→(pageviews, visitors, events).
func scanDailyTraffic(ctx context.Context, r *Repository, q string, from, to time.Time) (map[string][3]int64, error) {
	rows, err := r.db.Query(ctx, q, from, to)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := map[string][3]int64{}
	for rows.Next() {
		var d time.Time
		var v [3]int64
		if err := rows.Scan(&d, &v[0], &v[1], &v[2]); err != nil {
			return nil, err
		}
		out[dateDay(d)] = v
	}
	return out, rows.Err()
}

// scanDailySales returns map[date]→(gmv_cny, orders).
func scanDailySales(ctx context.Context, r *Repository, q string, from, to time.Time) (map[string][2]int64, error) {
	rows, err := r.db.Query(ctx, q, from, to)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := map[string][2]int64{}
	for rows.Next() {
		var d time.Time
		var v [2]int64
		if err := rows.Scan(&d, &v[0], &v[1]); err != nil {
			return nil, err
		}
		out[dateDay(d)] = v
	}
	return out, rows.Err()
}

// scanDailyCount returns map[date]→count for a single-name query.
func scanDailyCount(ctx context.Context, r *Repository, q, name string, from, to time.Time) (map[string]int64, error) {
	rows, err := r.db.Query(ctx, q, name, from, to)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := map[string]int64{}
	for rows.Next() {
		var d time.Time
		var n int64
		if err := rows.Scan(&d, &n); err != nil {
			return nil, err
		}
		out[dateDay(d)] = n
	}
	return out, rows.Err()
}

// scanDailyFunnelItin returns (submitted, confirmed) maps keyed by day.
func scanDailyFunnelItin(ctx context.Context, r *Repository, q string, from, to time.Time) (map[string]int64, map[string]int64, error) {
	rows, err := r.db.Query(ctx, q, from, to)
	if err != nil {
		return nil, nil, err
	}
	defer rows.Close()
	submitted := map[string]int64{}
	confirmed := map[string]int64{}
	for rows.Next() {
		var d time.Time
		var s, c int64
		if err := rows.Scan(&d, &s, &c); err != nil {
			return nil, nil, err
		}
		submitted[dateDay(d)] = s
		confirmed[dateDay(d)] = c
	}
	return submitted, confirmed, rows.Err()
}

// --- zero-fill (SQL gaps → contiguous daily series) --------------------------

// zeroFillTraffic builds a contiguous daily series from `from` (inclusive) to
// `to` (exclusive) in UTC days, filling absent days with zeros.
func zeroFillTraffic(from, to time.Time, m map[string][3]int64) []models.DailyPoint {
	var out []models.DailyPoint
	for d := from.UTC(); !d.Before(to.UTC()); d = d.AddDate(0, 0, 1) {
		k := dateDay(d)
		v := m[k]
		out = append(out, models.DailyPoint{
			Date:      k,
			Pageviews: v[0],
			Visitors:  v[1],
			Events:    v[2],
		})
	}
	return out
}

func zeroFillSales(from, to time.Time, m map[string][2]int64) []models.SalesDailyPoint {
	var out []models.SalesDailyPoint
	for d := from.UTC(); !d.Before(to.UTC()); d = d.AddDate(0, 0, 1) {
		k := dateDay(d)
		v := m[k]
		out = append(out, models.SalesDailyPoint{Date: k, GMVCny: v[0], Orders: v[1]})
	}
	return out
}

func zeroFillFunnel(from, to time.Time, views, submitted, confirmed map[string]int64) []models.FunnelDailyPoint {
	var out []models.FunnelDailyPoint
	for d := from.UTC(); !d.Before(to.UTC()); d = d.AddDate(0, 0, 1) {
		k := dateDay(d)
		out = append(out, models.FunnelDailyPoint{
			Date:      k,
			Views:     views[k],
			Submitted: submitted[k],
			Confirmed: confirmed[k],
		})
	}
	return out
}

// Ensure the pgx import is retained for the scan signatures (the row Scan
// path uses pgx types implicitly through the pool). `pgx` is imported for the
// ErrNoRows / rows helpers consistency with the ingest repo.
var _ = pgx.ErrNoRows
var _ *pgxpool.Pool
