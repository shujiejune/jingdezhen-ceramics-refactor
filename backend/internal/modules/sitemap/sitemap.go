// Package sitemap builds and serves the multi-locale sitemap.xml + robots.txt
// for international SEO (PRD §4.4, TDD §2.2).
//
// Slugs + meta columns already exist per-locale on the 4 content entities
// (products, ceramic_stories, activities, artists). This package joins the
// published translations across locales to emit:
//   - <urlset> with one <url> per (entity, locale, slug)
//   - <xhtml:link rel="alternate" hreflang="..."> alternates linking the
//     sibling published locales of the same entity (hreflang, per PRD §4.4)
//   - <loc> absolute URLs built from SITE_BASE_URL + locale prefix + segment
//   - <lastmod> from COALESCE(published_at, updated_at)
//
// The sitemap is rebuilt by the `sitemap:rebuild` Asynq job on content publish
// / unpublish (wired in the 4 content services) and is ALSO served fresh on
// GET /sitemap.xml (rebuild-on-read) so the file is always current. Rebuild-
// on-read is 4 small SELECTs — fine at MVP volume; a cache can be added later.
package sitemap

import (
	"bytes"
	"context"
	"encoding/xml"
	"fmt"
	"log"
	"net/url"
	"strings"
	"time"

	"jingdezhen-ceramics-backend/pkg/adapters/storage"

	"github.com/jackc/pgx/v5/pgxpool"
)

// Path segments for each public content entity in the SolidStart router
// (TDD §6). These mirror the API entity names; if the frontend chooses
// friendlier names (e.g. "history" for ceramicstory), edit these constants.
const (
	segProducts     = "products"
	segArtists      = "artists"
	segCeramicStory = "ceramicstory"
	segEngage       = "engage"
)

// sitemapKey is the storage object key the rebuild job writes (oss or local).
const sitemapKey = "sitemap.xml"

// Entity labels for the per-entity query dispatch.
const (
	entityProduct      = "product"
	entityCeramicStory = "ceramicstory"
	entityActivity     = "activity"
	entityArtist       = "artist"
)

// URLRow is one published translation row: the entity, its DB id (for
// grouping hreflang alternates), its locale + slug, and the lastmod timestamp.
type URLRow struct {
	Entity   string
	EntityID int64
	Locale   string
	Slug     string
	LastMod  time.Time
}

// Builder queries published content across the 4 entities and builds the
// sitemap XML. It writes the file to the storage adapter (OSS in prod, local
// in dev) via Rebuild; BuildXML returns the bytes for on-read serving.
type Builder struct {
	db          *pgxpool.Pool
	store       storage.Store
	siteBaseURL string
	// lister is the seam BuildXML uses; in prod it's the DB query
	// (listPublishedURLs). Unit tests inject a fake to assert XML shape without
	// a DB. nil → listPublishedURLs (the real DB path).
	lister func(ctx context.Context) ([]URLRow, error)
}

// NewBuilder constructs a Builder. siteBaseURL must be a valid absolute origin
// (e.g. "https://jingdezhen.example.com"); Rebuild/BuildXML return a clear
// error if it's empty/invalid so the job logs+skips rather than crashing.
func NewBuilder(db *pgxpool.Pool, store storage.Store, siteBaseURL string) *Builder {
	b := &Builder{db: db, store: store, siteBaseURL: siteBaseURL}
	b.lister = b.listPublishedURLs
	return b
}

// Rebuild builds the sitemap XML and writes it to the storage adapter. Called
// by the `sitemap:rebuild` Asynq job (worker) on content publish/unpublish.
func (b *Builder) Rebuild(ctx context.Context) error {
	xmlBytes, err := b.BuildXML(ctx)
	if err != nil {
		return err
	}
	if err := b.store.Put(ctx, sitemapKey, bytes.NewReader(xmlBytes), "application/xml; charset=utf-8"); err != nil {
		return fmt.Errorf("sitemap.Rebuild.Put: %w", err)
	}
	log.Printf("sitemap.Rebuild: wrote sitemap.xml (%d bytes)", len(xmlBytes))
	return nil
}

// BuildXML queries all published translations and returns the sitemap XML.
// Served directly by GET /sitemap.xml (always fresh) and also used by Rebuild.
func (b *Builder) BuildXML(ctx context.Context) ([]byte, error) {
	if b.siteBaseURL == "" {
		return nil, fmt.Errorf("sitemap.BuildXML: SITE_BASE_URL is empty (set SITE_BASE_URL env)")
	}
	if !strings.HasPrefix(b.siteBaseURL, "http://") && !strings.HasPrefix(b.siteBaseURL, "https://") {
		return nil, fmt.Errorf("sitemap.BuildXML: SITE_BASE_URL must be an absolute origin (got %q)", b.siteBaseURL)
	}

	rows, err := b.lister(ctx)
	if err != nil {
		return nil, fmt.Errorf("sitemap.BuildXML.list: %w", err)
	}

	// Group by (Entity, EntityID) to attach hreflang alternates per URL.
	// key = Entity + ":" + EntityID (int64), stable for map lookup.
	groups := map[string][]URLRow{}
	for _, r := range rows {
		k := r.Entity + ":" + fmt.Sprint(r.EntityID)
		groups[k] = append(groups[k], r)
	}

	type altLink struct {
		Hreflang string `xml:"hreflang,attr"`
		Href     string `xml:"href,attr"`
	}
	type urlEntry struct {
		Loc        string    `xml:"loc"`
		LastMod    string    `xml:"lastmod,omitempty"`
		Alternates []altLink `xml:"http://www.w3.org/1999/xhtml link"`
	}
	type urlset struct {
		XMLName xml.Name   `xml:"http://www.sitemaps.org/schemas/sitemap/0.9 urlset"`
		URLs    []urlEntry `xml:"url"`
	}

	set := urlset{URLs: []urlEntry{}}
	for _, group := range groups {
		// Build the alternate links once for the group (same set per URL in it).
		alts := make([]altLink, 0, len(group))
		for _, r := range group {
			alts = append(alts, altLink{
				Hreflang: hreflangFor(r.Locale),
				Href:     b.locFor(r.Entity, r.Locale, r.Slug),
			})
		}
		for _, r := range group {
			set.URLs = append(set.URLs, urlEntry{
				Loc:        b.locFor(r.Entity, r.Locale, r.Slug),
				LastMod:    r.LastMod.UTC().Format("2006-01-02T15:04:05Z"),
				Alternates: alts,
			})
		}
	}

	var buf bytes.Buffer
	buf.WriteString(xml.Header)
	enc := xml.NewEncoder(&buf)
	if err := enc.Encode(set); err != nil {
		return nil, fmt.Errorf("sitemap.BuildXML.Encode: %w", err)
	}
	return buf.Bytes(), nil
}

// locFor builds the absolute <loc> URL: SITE_BASE_URL/<locale>/<segment>/<slug>.
// locale is lowercased + "-"→"-" kept (BCP 47 in URL path is fine; the frontend
// may normalize further). Empty slug is defensive (shouldn't happen — slug is
// NOT NULL on every translation table).
func (b *Builder) locFor(entity, locale, slug string) string {
	seg := segmentFor(entity)
	// Lowercase the locale for the URL path (zh-CN → zh-cn) — cosmetic; the
	// SolidStart router validates against the supported list regardless.
	locLocale := strings.ToLower(locale)
	u := strings.TrimRight(b.siteBaseURL, "/") + "/" + url.PathEscape(locLocale) + "/" + seg + "/" + url.PathEscape(slug)
	return u
}

// hreflangFor maps a BCP 47 locale to its hreflang value. For MVP the launch
// locales are en-US + zh-CN, both valid hreflang values as-is.
func hreflangFor(locale string) string { return locale }

func segmentFor(entity string) string {
	switch entity {
	case entityProduct:
		return segProducts
	case entityCeramicStory:
		return segCeramicStory
	case entityActivity:
		return segEngage
	case entityArtist:
		return segArtists
	}
	return entity
}

// listPublishedURLs runs one small query per content entity and merges. Four
// queries are simpler than a UNION over mixed column names (story_id vs
// product_id vs …) and keep each query on a single index.
func (b *Builder) listPublishedURLs(ctx context.Context) ([]URLRow, error) {
	var all []URLRow
	for _, q := range publishedQueries {
		rows, err := b.db.Query(ctx, q.sql)
		if err != nil {
			return nil, fmt.Errorf("query %s: %w", q.entity, err)
		}
		for rows.Next() {
			var r URLRow
			if err := rows.Scan(&r.EntityID, &r.Locale, &r.Slug, &r.LastMod); err != nil {
				rows.Close()
				return nil, fmt.Errorf("scan %s: %w", q.entity, err)
			}
			r.Entity = q.entity
			all = append(all, r)
		}
		rows.Close()
		if err := rows.Err(); err != nil {
			return nil, fmt.Errorf("rows.Err %s: %w", q.entity, err)
		}
	}
	return all, nil
}

type publishedQuery struct {
	entity string
	sql    string
}

var publishedQueries = []publishedQuery{
	{entityProduct, `SELECT product_id, locale, slug, COALESCE(published_at, updated_at) AS lastmod FROM product_translations WHERE status = 'published'`},
	{entityCeramicStory, `SELECT story_id, locale, slug, COALESCE(published_at, updated_at) AS lastmod FROM ceramic_story_translations WHERE status = 'published'`},
	{entityActivity, `SELECT activity_id, locale, slug, COALESCE(published_at, updated_at) AS lastmod FROM activity_translations WHERE status = 'published'`},
	{entityArtist, `SELECT artist_id, locale, slug, COALESCE(published_at, updated_at) AS lastmod FROM artist_translations WHERE status = 'published'`},
}
