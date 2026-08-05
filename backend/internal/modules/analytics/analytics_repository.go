package analytics

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"

	"jingdezhen-ceramics-backend/internal/models"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// RepositoryInterface defines analytics storage operations.
type RepositoryInterface interface {
	// Insert stores one analytics event.
	Insert(ctx context.Context, e models.AnalyticsEvent) error
	// RollupPageviews aggregates the given date's pageviews into analytics_daily
	// (metric 'pageviews', dims {path,country,locale}). Idempotent: ON CONFLICT
	// … DO UPDATE SET value = excluded.value (set, not increment).
	RollupPageviews(ctx context.Context, date string) error
	// RollupEvents aggregates the given date's named events into analytics_daily
	// (metric 'events', dims {name,path,country,locale}).
	RollupEvents(ctx context.Context, date string) error
	// RollupVisitors aggregates unique visitor_hash per {country,locale} for
	// the date into analytics_daily (metric 'visitors', dims {country,locale}).
	RollupVisitors(ctx context.Context, date string) error
	// DailyExists is a tiny sanity hook for tests (the rollup must be idempotent
	// across re-runs for the same date).
	DailyCount(ctx context.Context, date string) (int, error)
}

type Repository struct {
	db *pgxpool.Pool
}

func NewRepository(db *pgxpool.Pool) RepositoryInterface {
	return &Repository{db: db}
}

func (r *Repository) Insert(ctx context.Context, e models.AnalyticsEvent) error {
	props, err := json.Marshal(e.Props)
	if err != nil {
		return fmt.Errorf("repository.Insert: marshal props: %w", err)
	}
	if e.Props == nil {
		props = []byte("{}")
	}
	const q = `
		INSERT INTO analytics_events (ts, kind, path, name, country, locale, visitor_hash, props)
		VALUES (COALESCE($1, NOW()), $2, $3, $4, $5, $6, $7, $8)`
	_, err = r.db.Exec(ctx, q, e.Ts, e.Kind, e.Path, e.Name, e.Country, e.Locale, e.VisitorHash, props)
	if err != nil {
		return fmt.Errorf("repository.Insert: %w", err)
	}
	return nil
}

// rollup runs an INSERT … ON CONFLICT (date, metric, dims) DO UPDATE that *sets*
// value from the recomputed source — so re-running the rollup for a date
// corrects the row rather than double-counting (idempotent).
func (r *Repository) rollup(ctx context.Context, date, metric, sourceSQL string) error {
	q := `
		INSERT INTO analytics_daily (date, metric, dims, value)
		SELECT $1::date, $2, dims, value FROM (
		` + sourceSQL + `
		) src
		ON CONFLICT (date, metric, dims) DO UPDATE SET value = excluded.value`
	_, err := r.db.Exec(ctx, q, date, metric)
	if err != nil {
		return fmt.Errorf("repository.rollup(%s): %w", metric, err)
	}
	return nil
}

func (r *Repository) RollupPageviews(ctx context.Context, date string) error {
	const src = `
		SELECT
			jsonb_build_object('path', path, 'country', country, 'locale', locale) AS dims,
			COUNT(*)::bigint AS value
		FROM analytics_events
		WHERE kind = 'pageview' AND ts::date = $1::date
		GROUP BY path, country, locale`
	return r.rollup(ctx, date, "pageviews", src)
}

func (r *Repository) RollupEvents(ctx context.Context, date string) error {
	const src = `
		SELECT
			jsonb_build_object('name', name, 'path', path, 'country', country, 'locale', locale) AS dims,
			COUNT(*)::bigint AS value
		FROM analytics_events
		WHERE kind = 'event' AND ts::date = $1::date
		GROUP BY name, path, country, locale`
	return r.rollup(ctx, date, "events", src)
}

func (r *Repository) RollupVisitors(ctx context.Context, date string) error {
	const src = `
		SELECT
			jsonb_build_object('country', country, 'locale', locale) AS dims,
			COUNT(DISTINCT visitor_hash)::bigint AS value
		FROM analytics_events
		WHERE ts::date = $1::date
		GROUP BY country, locale`
	return r.rollup(ctx, date, "visitors", src)
}

func (r *Repository) DailyCount(ctx context.Context, date string) (int, error) {
	const q = `SELECT COUNT(*) FROM analytics_daily WHERE date = $1::date`
	var n int
	err := r.db.QueryRow(ctx, q, date).Scan(&n)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return 0, nil
		}
		return 0, fmt.Errorf("repository.DailyCount: %w", err)
	}
	return n, nil
}
