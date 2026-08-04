package product_test

// Integration tests for the product repository against real PostgreSQL
// (testcontainers-go). These close out the TDD §11 priority list: i18n slug
// resolution + the tag filter.
//
// What these catch that unit tests can't:
//   - A regression in the UNIQUE(locale, slug) read path — e.g. a JOIN that
//     drops the locale filter and serves the wrong product to a locale.
//   - A regression in the tag-filter EXISTS subquery (written in the tags
//     feature, currently only manual-verified) — e.g. a typo that makes
//     ?tag= return all products or none.
//
// Pattern: NewDBPool(t) → seed minimal rows via the real repo (no SQL) →
// exercise + assert on rows re-read from the DB.

import (
	"context"
	"strconv"
	"sync/atomic"
	"testing"

	"jingdezhen-ceramics-backend/internal/models"
	"jingdezhen-ceramics-backend/internal/modules/product"
	"jingdezhen-ceramics-backend/internal/testutil"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/require"
)

// slugCounter keeps seeded slugs/sku-codes unique across tests so the
// UNIQUE(locale, slug) + UNIQUE(sku_code) constraints don't collide.
var slugCounter uint64

func uniqueSuffix() string { return strconv.FormatUint(atomic.AddUint64(&slugCounter, 1), 36) }

// createProduct inserts a product + its en-US published translation (and
// optional tags) via the real repo, returning the product id. The repo's
// CreateWithTranslation runs SetProductTags inside its tx, so tags are
// attached atomically — this is the same path the CMS uses.
func createProduct(t *testing.T, pool *pgxpool.Pool, slug string, tags []string) int64 {
	t.Helper()
	repo := product.NewRepository(pool)
	p, err := repo.CreateWithTranslation(context.Background(), models.CreateProductData{
		Locale: "en-US", Title: "P " + slug, Slug: slug, Tags: tags,
	})
	require.NoError(t, err)
	return p.ID
}

// TestFindPublishedBySlug_LocaleScoped verifies the i18n invariant (TDD §3.2):
// UNIQUE(locale, slug) means two locales can share the SAME slug text but
// resolve to DIFFERENT products. A regression that drops the locale filter
// from the JOIN would serve the wrong product to a locale — a silent i18n
// break (zh-CN site shows the en-US product, or 404s).
func TestFindPublishedBySlug_LocaleScoped(t *testing.T) {
	pool := testutil.NewDBPool(t)
	ctx := context.Background()
	repo := product.NewRepository(pool)

	// Two products, both published, both with slug "shared-slug" — but in
	// different locales. This is legal under UNIQUE(locale, slug) and is the
	// whole point of locale-scoped slugs (zh-CN and en-US can both have "vase").
	suf := uniqueSuffix()
	enSlug := "shared-slug-" + suf
	zhSlug := "shared-slug-" + suf // same text, different locale → different product
	enID := createProduct(t, pool, enSlug, nil)
	zhID := createProduct(t, pool, zhSlug+"-zh", nil) // zh needs its own en-US slug to insert

	// Publish the en-US translation (CreateWithTranslation leaves it as draft).
	_, err := pool.Exec(ctx, `UPDATE product_translations SET status='published', published_at=NOW()
		WHERE product_id=$1 AND locale='en-US'`, enID)
	require.NoError(t, err)
	_, err = pool.Exec(ctx, `UPDATE product_translations SET status='published', published_at=NOW()
		WHERE product_id=$1 AND locale='en-US'`, zhID)
	require.NoError(t, err)
	// Add a zh-CN translation to the second product WITH the shared slug text.
	_, err = pool.Exec(ctx, `INSERT INTO product_translations (product_id, locale, title, slug, status, published_at)
		VALUES ($1,'zh-CN','中文产品',$2,'published',NOW())`, zhID, zhSlug)
	require.NoError(t, err)
	// And a zh-CN translation to the first product with a DIFFERENT slug (so
	// both locales exist on product 1, but the shared text resolves to product 2
	// in zh-CN).
	_, err = pool.Exec(ctx, `INSERT INTO product_translations (product_id, locale, title, slug, status, published_at)
		VALUES ($1,'zh-CN','产品一','other-zh','published',NOW())`, enID)
	require.NoError(t, err)

	// en-US lookup of the shared slug → product 1 (the en-US row).
	gotEN, err := repo.FindPublishedBySlug(ctx, "en-US", enSlug)
	require.NoError(t, err)
	require.Equal(t, enID, gotEN.ID, "en-US slug must resolve to the en-US product")

	// zh-CN lookup of the SAME slug text → product 2 (the zh-CN row). This is
	// the invariant: same text, different locale, different product.
	gotZH, err := repo.FindPublishedBySlug(ctx, "zh-CN", zhSlug)
	require.NoError(t, err)
	require.Equal(t, zhID, gotZH.ID, "zh-CN slug must resolve to the zh-CN product, not the en-US one")

	// en-US lookup of a slug that only exists in zh-CN → NotFound (locale filter
	// is enforced, no cross-locale fallback on the slug path).
	_, err = repo.FindPublishedBySlug(ctx, "en-US", "other-zh")
	require.ErrorIs(t, err, models.ErrNotFound, "a slug absent in the requested locale must 404, not fall back")
}

// TestFindPublishedBySlug_OnlyPublished verifies the status filter: a draft or
// rejected translation is invisible on the public read path even if the slug
// matches. A regression here would leak unpublished content to the catalog.
func TestFindPublishedBySlug_OnlyPublished(t *testing.T) {
	pool := testutil.NewDBPool(t)
	ctx := context.Background()
	repo := product.NewRepository(pool)

	draftSlug := "draft-slug-" + uniqueSuffix()
	draftID := createProduct(t, pool, draftSlug, nil)
	// Leave it as draft (CreateWithTranslation default).
	_, err := pool.Exec(ctx, `UPDATE product_translations SET status='draft'
		WHERE product_id=$1 AND locale='en-US'`, draftID)
	require.NoError(t, err)

	_, err = repo.FindPublishedBySlug(ctx, "en-US", draftSlug)
	require.ErrorIs(t, err, models.ErrNotFound, "a draft translation must not be publicly visible")
}

// TestFindAllPublished_TagFilter verifies the tag-filter EXISTS subquery
// written in the tags feature (currently only manual-verified). Three products:
// one tagged [hand-painted], one tagged [celadon-glaze], one untagged. The
// filter must return only the products carrying the requested tag (ANY-match
// for multi-tag), and an unknown tag returns zero products.
func TestFindAllPublished_TagFilter(t *testing.T) {
	pool := testutil.NewDBPool(t)
	ctx := context.Background()
	repo := product.NewRepository(pool)

	taggedA := createProduct(t, pool, "tagged-a-"+uniqueSuffix(), []string{"hand-painted"})
	taggedB := createProduct(t, pool, "tagged-b-"+uniqueSuffix(), []string{"celadon-glaze", "hand-painted"})
	untagged := createProduct(t, pool, "untagged-"+uniqueSuffix(), nil)

	// Publish all three.
	for _, id := range []int64{taggedA, taggedB, untagged} {
		_, err := pool.Exec(ctx, `UPDATE product_translations SET status='published', published_at=NOW()
			WHERE product_id=$1 AND locale='en-US'`, id)
		require.NoError(t, err)
	}

	// Filter by single tag → both hand-painted products (A + B), not the
	// untagged one.
	got, total, err := repo.FindAllPublished(ctx, "en-US", "", 0, []string{"hand-painted"}, 1, 50)
	require.NoError(t, err)
	require.Equal(t, 2, total)
	ids := productIDs(got)
	require.ElementsMatch(t, []int64{taggedA, taggedB}, ids, "hand-painted filter must return A+B only")

	// Filter by a tag only B has → just B.
	got, total, err = repo.FindAllPublished(ctx, "en-US", "", 0, []string{"celadon-glaze"}, 1, 50)
	require.NoError(t, err)
	require.Equal(t, 1, total)
	require.Equal(t, taggedB, got[0].ID)

	// Multi-tag (ANY-match): hand-painted OR celadon-glaze → still A+B (B has
	// both but is counted once; A has hand-painted).
	got, total, err = repo.FindAllPublished(ctx, "en-US", "", 0, []string{"hand-painted", "celadon-glaze"}, 1, 50)
	require.NoError(t, err)
	require.Equal(t, 2, total)

	// Unknown tag → zero products (the EXISTS subquery matches nothing).
	got, total, err = repo.FindAllPublished(ctx, "en-US", "", 0, []string{"nonexistent-tag"}, 1, 50)
	require.NoError(t, err)
	require.Zero(t, total)
	require.Empty(t, got)

	// No tag filter → all three (the filter is additive, not required).
	got, total, err = repo.FindAllPublished(ctx, "en-US", "", 0, nil, 1, 50)
	require.NoError(t, err)
	require.Equal(t, 3, total)
}

// TestFindAllTagsInUse_OnlyPublishedProducts verifies the facet list only
// counts tags attached to ≥1 PUBLISHED product. A tag on a draft product must
// not appear in the public facet list (it would advertise unpublished content).
func TestFindAllTagsInUse_OnlyPublishedProducts(t *testing.T) {
	pool := testutil.NewDBPool(t)
	ctx := context.Background()
	repo := product.NewRepository(pool)

	// Published product with a tag.
	pubID := createProduct(t, pool, "pub-tagged-"+uniqueSuffix(), []string{"visible-tag"})
	_, err := pool.Exec(ctx, `UPDATE product_translations SET status='published', published_at=NOW()
		WHERE product_id=$1 AND locale='en-US'`, pubID)
	require.NoError(t, err)
	// Draft product with a different tag.
	draftID := createProduct(t, pool, "draft-tagged-"+uniqueSuffix(), []string{"hidden-tag"})
	_, err = pool.Exec(ctx, `UPDATE product_translations SET status='draft'
		WHERE product_id=$1 AND locale='en-US'`, draftID)
	require.NoError(t, err)

	tags, err := repo.FindAllTagsInUse(ctx, "en-US")
	require.NoError(t, err)

	keys := make([]string, 0, len(tags))
	for _, tg := range tags {
		keys = append(keys, tg.Key)
	}
	require.Contains(t, keys, "visible-tag", "tag on a published product must appear in the facet list")
	require.NotContains(t, keys, "hidden-tag", "tag on a draft-only product must NOT appear (would leak unpublished content)")
}

func productIDs(ps []models.Product) []int64 {
	out := make([]int64, len(ps))
	for i, p := range ps {
		out[i] = p.ID
	}
	return out
}
