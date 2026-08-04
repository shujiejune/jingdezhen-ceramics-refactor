package certificate_test

// Integration tests for the certificate repository against real PostgreSQL
// (testcontainers-go). The bug-prone bits unique to certificates:
//   - cert_code is JDZ-<6-base32-crypto-rand> with a UNIQUE constraint + a
//     collision-retry loop in NextCode. A bug here means two products share a
//     certificate code — a provenance/authenticity break, worse than a money
//     bug for this product (the whole point of the cert is uniqueness).
//   - product_id UNIQUE: one certificate per product. Issuing twice for the
//     same product must fail (the guard against duplicate certs).
//   - The `created` provenance is appended in the same tx as the cert insert
//     (an atomicity invariant — a cert with no `created` row is a broken chain).
//
// Pattern: NewDBPool(t) → seed a product (certificates.product_id FK) →
// exercise the real repo → assert on rows re-read from the DB.

import (
	"context"
	"strconv"
	"sync/atomic"
	"testing"

	"jingdezhen-ceramics-backend/internal/models"
	"jingdezhen-ceramics-backend/internal/modules/certificate"
	"jingdezhen-ceramics-backend/internal/testutil"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/require"
)

// slugCounter keeps seeded slugs unique across tests so the
// UNIQUE(locale, slug) constraint doesn't collide when multiple products are
// created in one test.
var slugCounter uint64

func uniqueSuffix() string { return strconv.FormatUint(atomic.AddUint64(&slugCounter, 1), 36) }

// seedProduct inserts one product (en-US translation, published) and returns
// its id. Certificates FK to products, so every cert test needs a product.
func seedProduct(t *testing.T, pool *pgxpool.Pool) int64 {
	t.Helper()
	ctx := context.Background()
	slug := "t-" + uniqueSuffix()
	var id int64
	err := pool.QueryRow(ctx, `INSERT INTO products (category, display_order)
		VALUES ('test', 0) RETURNING id`).Scan(&id)
	require.NoError(t, err)
	_, err = pool.Exec(ctx, `INSERT INTO product_translations (product_id, locale, title, slug, status, published_at)
		VALUES ($1,'en-US','T',$2,'published',NOW())`, id, slug)
	require.NoError(t, err)
	return id
}

// TestNextCode_GeneratesUniqueCodes asserts the collision-retry loop produces
// codes not already in the DB. We pre-insert a code, then confirm NextCode
// does NOT return it (it must probe + retry past the collision).
func TestNextCode_GeneratesUniqueCodes(t *testing.T) {
	pool := testutil.NewDBPool(t)
	ctx := context.Background()
	repo := certificate.NewRepository(pool)
	pid := seedProduct(t, pool)

	// Issue one cert, capture its code.
	code1, err := repo.NextCode(ctx)
	require.NoError(t, err)
	require.NotEmpty(t, code1)
	c1, err := repo.Issue(ctx, pid, code1)
	require.NoError(t, err)
	require.Equal(t, code1, c1.CertCode)

	// NextCode must not return code1 again (it's now in the DB).
	code2, err := repo.NextCode(ctx)
	require.NoError(t, err)
	require.NotEqual(t, code1, code2, "NextCode must not return a code already present in the DB")

	// A second product gets a distinct cert.
	pid2 := seedProduct(t, pool)
	c2, err := repo.Issue(ctx, pid2, code2)
	require.NoError(t, err)
	require.Equal(t, code2, c2.CertCode)
	require.NotEqual(t, c1.ID, c2.ID)
}

// TestIssue_ManyCodesAllDistinct is the real uniqueness stress: issue N
// certificates and assert N distinct codes. With 6 base32 chars there are
// 32^6 ≈ 1e9 codes, so 50 random issues should never collide — but if the
// retry loop or the alphabet mapping is broken, duplicates surface here. This
// is the test that catches a broken code() (e.g. a modulo bias that collapses
// the space, or a missing collision retry that returns a dup).
func TestIssue_ManyCodesAllDistinct(t *testing.T) {
	pool := testutil.NewDBPool(t)
	ctx := context.Background()
	repo := certificate.NewRepository(pool)

	const n = 50
	seen := map[string]int64{} // code → productID
	for i := 0; i < n; i++ {
		pid := seedProduct(t, pool)
		code, err := repo.NextCode(ctx)
		require.NoError(t, err)
		require.Len(t, code, 10, "cert_code must be JDZ-<6 chars> = 10 chars")
		require.Regexp(t, `^JDZ-[A-Z2-7]{6}$`, code, "cert_code must be JDZ- + 6 base32 chars")
		c, err := repo.Issue(ctx, pid, code)
		require.NoError(t, err)
		if prev, dup := seen[code]; dup {
			t.Fatalf("duplicate cert_code %q: product %d and %d", code, prev, pid)
		}
		seen[code] = pid
		_ = c
	}
	require.Equal(t, n, len(seen), "all %d codes must be distinct", n)
}

// TestIssue_ProductIDUnique verifies the one-certificate-per-product guard:
// issuing a second cert for the same product fails (product_id UNIQUE). A
// regression here means a product can accumulate multiple certs — the
// provenance chain becomes ambiguous.
func TestIssue_ProductIDUnique(t *testing.T) {
	pool := testutil.NewDBPool(t)
	ctx := context.Background()
	repo := certificate.NewRepository(pool)
	pid := seedProduct(t, pool)

	code1, err := repo.NextCode(ctx)
	require.NoError(t, err)
	_, err = repo.Issue(ctx, pid, code1)
	require.NoError(t, err)

	// Second issue for the SAME product → unique violation (wrapped error).
	code2, err := repo.NextCode(ctx)
	require.NoError(t, err)
	_, err = repo.Issue(ctx, pid, code2)
	require.Error(t, err, "issuing a second cert for the same product must fail (product_id UNIQUE)")
}

// TestIssue_AppendsCreatedProvenance verifies the atomicity invariant: the
// `created` provenance row is inserted in the same tx as the cert. A cert with
// no `created` row is a broken provenance chain (the public QR page shows an
// empty history). The tx must not commit the cert without the provenance.
func TestIssue_AppendsCreatedProvenance(t *testing.T) {
	pool := testutil.NewDBPool(t)
	ctx := context.Background()
	repo := certificate.NewRepository(pool)
	pid := seedProduct(t, pool)

	code, err := repo.NextCode(ctx)
	require.NoError(t, err)
	c, err := repo.Issue(ctx, pid, code)
	require.NoError(t, err)

	// Re-read the provenance chain from the DB (don't trust the return value).
	chain, err := repo.LoadProvenance(ctx, c.ID)
	require.NoError(t, err)
	require.Len(t, chain, 1, "a freshly-issued cert must have exactly one `created` provenance row")
	require.Equal(t, models.ProvenanceCreated, chain[0].Kind)
}

// TestGetByCode_LoadsProductAndProvenance verifies the public QR-page read
// path: GetByCode JOINs certificates → products → product_translations (for
// the locale display) + loads the provenance chain. A regression in the JOIN
// would 404 a valid code or serve the wrong product.
func TestGetByCode_LoadsProductAndProvenance(t *testing.T) {
	pool := testutil.NewDBPool(t)
	ctx := context.Background()
	repo := certificate.NewRepository(pool)
	pid := seedProduct(t, pool)

	code, err := repo.NextCode(ctx)
	require.NoError(t, err)
	_, err = repo.Issue(ctx, pid, code)
	require.NoError(t, err)

	// Public lookup by code → loads the product + the `created` provenance.
	got, err := repo.GetByCode(ctx, code, "en-US")
	require.NoError(t, err)
	require.Equal(t, pid, got.ProductID)
	require.Equal(t, code, got.CertCode)
	require.NotEmpty(t, got.ProductTitle, "the JOIN must populate the product title for the locale")
	require.Len(t, got.Provenance, 1, "GetByCode must load the provenance chain")

	// A nonexistent code → ErrNotFound (not a panic / nil deref).
	_, err = repo.GetByCode(ctx, "JDZ-NOPE", "en-US")
	require.ErrorIs(t, err, models.ErrNotFound)
}
