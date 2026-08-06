// Package sitemap provides the multi-locale sitemap.xml + robots.txt for SEO
// (PRD §4.4). This file holds the shared hreflang-alternates helper used by
// the 4 content services' detail handlers to populate the Alternates map on
// their response DTOs.
//
// Rather than add a FindPublishedSlugsByID method to each of the 4 content
// repositories (expanding 4 interfaces for one SEO field), the services call
// FindAlternates — one function per entity table, taking the pgx executor.
// The executor is the pool in serve/worker; a tx in tests if needed.

package sitemap

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5"
)

// FindAlternates returns the locale→slug map of all published translations
// for one entity, EXCLUDING the current locale (the current locale's slug is
// already in the response; alternates are the *other* locales). Returns an
// empty map if the entity has only one published locale. The frontend emits
// <link rel="alternate" hreflang="..."> from this map (PRD §4.4).
//
// entityID=0 returns an empty map (defensive — a not-yet-saved entity).
func FindAlternates(ctx context.Context, db pgxExecutor, table, idCol string, entityID int64, currentLocale string) (map[string]string, error) {
	if entityID == 0 {
		return map[string]string{}, nil
	}
	q := fmt.Sprintf(
		`SELECT locale, slug FROM %s WHERE %s = $1 AND status = 'published' AND locale <> $2`,
		table, idCol,
	)
	rows, err := db.Query(ctx, q, entityID, currentLocale)
	if err != nil {
		return nil, fmt.Errorf("sitemap.FindAlternates(%s): %w", table, err)
	}
	defer rows.Close()
	out := map[string]string{}
	for rows.Next() {
		var locale, slug string
		if err := rows.Scan(&locale, &slug); err != nil {
			return nil, fmt.Errorf("sitemap.FindAlternates(%s) scan: %w", table, err)
		}
		out[locale] = slug
	}
	return out, rows.Err()
}

// pgxExecutor is the minimal subset of pgxpool.Pool / pgx.Tx that
// FindAlternates needs. Both *pgxpool.Pool and pgx.Tx implement Query.
type pgxExecutor interface {
	Query(ctx context.Context, sql string, args ...any) (pgx.Rows, error)
}

// Per-entity wrappers: the services call these to avoid passing table/column
// names as strings (typed convenience; the table+id-column are baked here).

// ProductAlternates returns alternates for a product.
func ProductAlternates(ctx context.Context, db pgxExecutor, productID int64, currentLocale string) (map[string]string, error) {
	return FindAlternates(ctx, db, "product_translations", "product_id", productID, currentLocale)
}

// CeramicStoryAlternates returns alternates for a ceramic story.
func CeramicStoryAlternates(ctx context.Context, db pgxExecutor, storyID int64, currentLocale string) (map[string]string, error) {
	return FindAlternates(ctx, db, "ceramic_story_translations", "story_id", storyID, currentLocale)
}

// ActivityAlternates returns alternates for an activity.
func ActivityAlternates(ctx context.Context, db pgxExecutor, activityID int64, currentLocale string) (map[string]string, error) {
	return FindAlternates(ctx, db, "activity_translations", "activity_id", activityID, currentLocale)
}

// ArtistAlternates returns alternates for an artist.
func ArtistAlternates(ctx context.Context, db pgxExecutor, artistID int64, currentLocale string) (map[string]string, error) {
	return FindAlternates(ctx, db, "artist_translations", "artist_id", artistID, currentLocale)
}
