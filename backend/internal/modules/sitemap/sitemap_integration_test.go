package sitemap_test

import (
	"context"
	"encoding/xml"
	"io"
	"strings"
	"testing"

	"jingdezhen-ceramics-backend/internal/modules/sitemap"
	"jingdezhen-ceramics-backend/internal/testutil"
	"jingdezhen-ceramics-backend/pkg/adapters/storage"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Integration tests hit a real, fully-migrated PostgreSQL (testcontainers via
// testutil.NewDBPool). They verify the DB-backed listPublishedURLs query +
// the FindAlternates helper against the real translation tables.

// fakeStore records the last Put (Rebuild). Local-only; no FS.
type fakeStore struct {
	putBody []byte
}

func (f *fakeStore) Mode() string { return "local" }
func (f *fakeStore) PresignUpload(context.Context, string, string, int64) (string, map[string]string, error) {
	return "", nil, nil
}
func (f *fakeStore) PublicURL(string) string { return "" }
func (f *fakeStore) Put(_ context.Context, _ string, r io.Reader, _ string) error {
	buf, _ := io.ReadAll(r)
	f.putBody = buf
	return nil
}
func (f *fakeStore) Delete(context.Context, string) error     { return nil }
func (f *fakeStore) Key(storage.Kind, string) (string, error) { return "", nil }

const siteBase = "https://jingdezhen.example.com"

// TestBuildXML_TwoLocaleProduct_PublishedBoth proves the DB query + hreflang
// alternates: a product with 2 published locale translations → 2 <url>s, each
// with 2 alternates, both absolute, locale-segment-slug correct.
func TestBuildXML_TwoLocaleProduct_PublishedBoth(t *testing.T) {
	if testing.Short() {
		t.Skip("integration: needs testcontainers PG")
	}
	pool := testutil.NewDBPool(t)
	ctx := context.Background()

	// Seed an artist (products need an artist FK? not always; use a no-artist product).
	// Insert a product + 2 published translations with distinct en/zh slugs.
	productID := seedProductWithTwoLocales(t, pool, "blue-vase-en", "lan-hua-ping-zh")

	b := sitemap.NewBuilder(pool, &fakeStore{}, siteBase)
	xmlBytes, err := b.BuildXML(ctx)
	require.NoError(t, err)

	set := mustParseXML(t, xmlBytes)
	// Exactly 2 <url> entries for this product (en-US + zh-CN).
	var productURLs []iurlEntry
	for _, u := range set.URLs {
		if strings.Contains(u.Loc, "/products/blue-vase-en") || strings.Contains(u.Loc, "/products/lan-hua-ping-zh") {
			productURLs = append(productURLs, u)
		}
	}
	require.Len(t, productURLs, 2, "one <url> per published locale")
	for _, u := range productURLs {
		assert.Len(t, u.Alternates, 2, "both published locales → 2 hreflang alternates")
		assert.True(t, strings.HasPrefix(u.Loc, siteBase+"/"), "absolute loc: %s", u.Loc)
		assert.NotEmpty(t, u.LastMod)
	}

	// FindPublishedAlternates must return the OTHER locale's slug (excludes current).
	enAlts, err := sitemap.FindAlternates(ctx, pool, "product_translations", "product_id", productID, "en-US")
	require.NoError(t, err)
	assert.Equal(t, map[string]string{"zh-CN": "lan-hua-ping-zh"}, enAlts)
	zhAlts, err := sitemap.FindAlternates(ctx, pool, "product_translations", "product_id", productID, "zh-CN")
	require.NoError(t, err)
	assert.Equal(t, map[string]string{"en-US": "blue-vase-en"}, zhAlts)
}

// TestBuildXML_UnpublishedExcluded proves an unpublished translation is absent
// from the sitemap + absent from a sibling's alternates.
func TestBuildXML_UnpublishedExcluded(t *testing.T) {
	if testing.Short() {
		t.Skip("integration: needs testcontainers PG")
	}
	pool := testutil.NewDBPool(t)
	ctx := context.Background()

	productID := seedProductTwoLocalesOnePublished(t, pool, "pub-en", "draft-zh")

	b := sitemap.NewBuilder(pool, &fakeStore{}, siteBase)
	xmlBytes, err := b.BuildXML(ctx)
	require.NoError(t, err)

	set := mustParseXML(t, xmlBytes)
	// Only the published en-US URL appears.
	var found bool
	for _, u := range set.URLs {
		if strings.Contains(u.Loc, "/products/draft-zh") {
			t.Fatalf("unpublished zh-CN slug appeared in sitemap: %s", u.Loc)
		}
		if strings.Contains(u.Loc, "/products/pub-en") {
			found = true
			assert.Len(t, u.Alternates, 1, "only 1 published locale → only the self alternate")
		}
	}
	assert.True(t, found, "published en-US URL present")

	// FindAlternates for the en-US locale → empty (zh-CN is draft, excluded).
	enAlts, err := sitemap.FindAlternates(ctx, pool, "product_translations", "product_id", productID, "en-US")
	require.NoError(t, err)
	assert.Empty(t, enAlts, "draft zh-CN is not an alternate")
}

// TestRebuild_WritesStore proves Rebuild writes non-empty XML to the store.
func TestRebuild_WritesStore(t *testing.T) {
	if testing.Short() {
		t.Skip("integration: needs testcontainers PG")
	}
	pool := testutil.NewDBPool(t)
	ctx := context.Background()
	seedProductWithTwoLocales(t, pool, "rv-en", "rv-zh")

	fs := &fakeStore{}
	b := sitemap.NewBuilder(pool, fs, siteBase)
	require.NoError(t, b.Rebuild(ctx))
	assert.True(t, strings.HasPrefix(string(fs.putBody), "<?xml"), "store received XML")
	assert.Contains(t, string(fs.putBody), "/products/rv-en")
}

// seedProductWithTwoLocales inserts a product + 2 PUBLISHED translations
// (en-US slug, zh-CN slug) and returns the product id.
func seedProductWithTwoLocales(t *testing.T, pool *pgxpool.Pool, enSlug, zhSlug string) int64 {
	t.Helper()
	ctx := context.Background()
	var id int64
	err := pool.QueryRow(ctx, `INSERT INTO products (created_at, updated_at) VALUES (NOW(), NOW()) RETURNING id`).Scan(&id)
	require.NoError(t, err)
	mustExec := func(sql string, args ...any) {
		_, err := pool.Exec(ctx, sql, args...)
		require.NoError(t, err)
	}
	mustExec(`INSERT INTO product_translations (product_id, locale, title, slug, status, published_at, updated_at)
		VALUES ($1, 'en-US', 'Blue Vase', $2, 'published', NOW(), NOW())`, id, enSlug)
	mustExec(`INSERT INTO product_translations (product_id, locale, title, slug, status, published_at, updated_at)
		VALUES ($1, 'zh-CN', '蓝花瓶', $2, 'published', NOW(), NOW())`, id, zhSlug)
	return id
}

func seedProductTwoLocalesOnePublished(t *testing.T, pool *pgxpool.Pool, enSlug, zhSlug string) int64 {
	t.Helper()
	ctx := context.Background()
	var id int64
	err := pool.QueryRow(ctx, `INSERT INTO products (created_at, updated_at) VALUES (NOW(), NOW()) RETURNING id`).Scan(&id)
	require.NoError(t, err)
	mustExec := func(sql string, args ...any) {
		_, err := pool.Exec(ctx, sql, args...)
		require.NoError(t, err)
	}
	mustExec(`INSERT INTO product_translations (product_id, locale, title, slug, status, published_at, updated_at)
		VALUES ($1, 'en-US', 'Pub', $2, 'published', NOW(), NOW())`, id, enSlug)
	mustExec(`INSERT INTO product_translations (product_id, locale, title, slug, status, updated_at)
		VALUES ($1, 'zh-CN', 'Draft', $2, 'draft', NOW())`, id, zhSlug)
	return id
}

// --- XML parse helpers (this file is package sitemap_test, separate from
// sitemap_test.go's package sitemap, so re-declare locally). ---
func mustParseXML(t *testing.T, b []byte) iurlSet {
	t.Helper()
	var set iurlSet
	dec := xml.NewDecoder(strings.NewReader(string(b)))
	dec.Strict = false
	require.NoError(t, dec.Decode(&set), "malformed sitemap XML: %s", string(b))
	return set
}

type ialtLink struct {
	Hreflang string `xml:"hreflang,attr"`
	Href     string `xml:"href,attr"`
}
type iurlEntry struct {
	Loc        string     `xml:"loc"`
	LastMod    string     `xml:"lastmod,omitempty"`
	Alternates []ialtLink `xml:"http://www.w3.org/1999/xhtml link"`
}
type iurlSet struct {
	XMLName xml.Name    `xml:"urlset"`
	URLs    []iurlEntry `xml:"url"`
}
