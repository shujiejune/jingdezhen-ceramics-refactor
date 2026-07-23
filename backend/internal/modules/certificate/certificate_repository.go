package certificate

import (
	"context"
	"crypto/rand"
	"encoding/json"
	"fmt"
	"jingdezhen-ceramics-backend/internal/models"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// RepositoryInterface defines certificate + provenance storage.
type RepositoryInterface interface {
	// Issue inserts a certificate for a product + a `created` provenance row
	// in one transaction. Returns ErrConflict if a certificate already exists
	// for the product (product_id UNIQUE).
	Issue(ctx context.Context, productID int64, code string) (*models.Certificate, error)
	// GetByCode loads a certificate + the product/artist display info for the
	// requested locale (published-or-fallback translation) + the provenance
	// chain ordered by time. Returns ErrNotFound if the code doesn't exist.
	GetByCode(ctx context.Context, code, locale string) (*models.Certificate, error)
	// GetByProductID loads a certificate by its product (admin).
	GetByProductID(ctx context.Context, productID int64) (*models.Certificate, error)
	// GetByID loads a certificate by id (admin).
	GetByID(ctx context.Context, id int64) (*models.Certificate, error)
	// ListAdmin paginates all certificates (admin).
	ListAdmin(ctx context.Context, page, limit int) ([]models.Certificate, int, error)
	// RegenerateCode issues a new cert_code for an existing certificate + a
	// `created` provenance row with detail {regenerated:true}. Returns the new code.
	RegenerateCode(ctx context.Context, id int64) (string, error)
	// AppendProvenance appends a provenance record to a certificate.
	AppendProvenance(ctx context.Context, certificateID int64, kind models.ProvenanceKind, detail json.RawMessage) error
	// LoadProvenance returns the provenance chain for a certificate (admin detail).
	LoadProvenance(ctx context.Context, certificateID int64) ([]models.ProvenanceRecord, error)
	// NextCode generates a unique cert_code (collision retry). Exported for the
	// service to generate the code before Issue (so the certchain adapter can
	// register it before the insert).
	NextCode(ctx context.Context) (string, error)
	// FindCertsBySKUs maps sku_id → certificate for the sale path. JOINs
	// certificates → products → skus so the caller passes SKU ids (which
	// order_items snapshots) and gets the product's cert back.
	FindCertsBySKUs(ctx context.Context, skuIDs []int64) (map[int64]*models.Certificate, error)
}

type Repository struct {
	db *pgxpool.Pool
}

func NewRepository(db *pgxpool.Pool) RepositoryInterface {
	return &Repository{db: db}
}

// code generates a unique certificate code: JDZ-<6 base32 chars from crypto/rand>.
// The UNIQUE constraint collision is astronomically unlikely (32^6 ≈ 1e9); the
// service retries on a collision.
func code() string {
	const alphabet = "ABCDEFGHIJKLMNOPQRSTUVWXYZ234567" // base32 (no 0/1/I/O ambiguity)
	b := make([]byte, 6)
	if _, err := rand.Read(b); err != nil {
		panic("crypto/rand failed: " + err.Error()) // unrecoverable
	}
	for i := range b {
		b[i] = alphabet[int(b[i])%len(alphabet)]
	}
	return "JDZ-" + string(b)
}

// NextCode generates a code not already present in the DB (collision retry).
func (r *Repository) NextCode(ctx context.Context) (string, error) {
	for i := 0; i < 5; i++ {
		c := code()
		var exists bool
		if err := r.db.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM certificates WHERE cert_code=$1)`, c).Scan(&exists); err != nil {
			return "", fmt.Errorf("certificate.nextCode.probe: %w", err)
		}
		if !exists {
			return c, nil
		}
	}
	return "", fmt.Errorf("certificate.nextCode: could not generate a unique code after 5 tries")
}

func (r *Repository) Issue(ctx context.Context, productID int64, c string) (*models.Certificate, error) {
	tx, err := r.db.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return nil, fmt.Errorf("certificate.Issue.BeginTx: %w", err)
	}
	defer tx.Rollback(ctx) //nolint:errcheck

	var cert models.Certificate
	if err := tx.QueryRow(ctx, `
		INSERT INTO certificates (product_id, cert_code)
		VALUES ($1, $2)
		RETURNING id, product_id, cert_code, qr_key, pdf_key, issued_at, created_at, updated_at`,
		productID, c,
	).Scan(&cert.ID, &cert.ProductID, &cert.CertCode, &cert.QRKey, &cert.PDFKey,
		&cert.IssuedAt, &cert.CreatedAt, &cert.UpdatedAt); err != nil {
		return nil, fmt.Errorf("certificate.Issue.Insert: %w", err)
	}
	detail, _ := json.Marshal(map[string]any{"issued": true})
	if _, err := tx.Exec(ctx, `
		INSERT INTO provenance_records (certificate_id, kind, detail)
		VALUES ($1, 'created', $2)`, cert.ID, detail); err != nil {
		return nil, fmt.Errorf("certificate.Issue.Provenance: %w", err)
	}
	if err := tx.Commit(ctx); err != nil {
		return nil, fmt.Errorf("certificate.Issue.Commit: %w", err)
	}
	return &cert, nil
}

const certSelectCols = `
	c.id, c.product_id, c.cert_code, c.qr_key, c.pdf_key, c.issued_at, c.created_at, c.updated_at `

func (r *Repository) scanCert(row pgx.Row) (*models.Certificate, error) {
	var c models.Certificate
	if err := row.Scan(&c.ID, &c.ProductID, &c.CertCode, &c.QRKey, &c.PDFKey,
		&c.IssuedAt, &c.CreatedAt, &c.UpdatedAt); err != nil {
		return nil, err
	}
	return &c, nil
}

func (r *Repository) LoadProvenance(ctx context.Context, certID int64) ([]models.ProvenanceRecord, error) {
	rows, err := r.db.Query(ctx, `
		SELECT id, certificate_id, kind, detail, at
		FROM provenance_records WHERE certificate_id = $1 ORDER BY at ASC, id ASC`, certID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []models.ProvenanceRecord{}
	for rows.Next() {
		var p models.ProvenanceRecord
		if err := rows.Scan(&p.ID, &p.CertificateID, &p.Kind, &p.Detail, &p.At); err != nil {
			return nil, err
		}
		out = append(out, p)
	}
	return out, rows.Err()
}

// GetByCode loads a certificate + product/artist display info (locale-aware,
// published translation with fallback to any) + the provenance chain.
func (r *Repository) GetByCode(ctx context.Context, codeStr, locale string) (*models.Certificate, error) {
	// Join certificate → product → product_translations (locale, published or
	// fallback) → artists. The cert is the authenticity proof, so it shows even
	// if the product translation is unpublished (fall back to en-US or any).
	query := `
		SELECT ` + certSelectCols + `,
		       COALESCE(pt.title, pt2.title) AS title,
		       COALESCE(pt.slug, pt2.slug) AS slug,
		       a.name, a.id,
		       p.thumbnail_url,
		       (SELECT attributes FROM skus WHERE product_id = p.id ORDER BY id LIMIT 1)
		FROM certificates c
		JOIN products p ON p.id = c.product_id
		LEFT JOIN product_translations pt ON pt.product_id = p.id AND pt.locale = $2
		LEFT JOIN product_translations pt2 ON pt2.product_id = p.id AND pt2.locale = 'en-US'
		LEFT JOIN artists a ON a.id = p.artist_id
		WHERE c.cert_code = $1`
	var (
		c       models.Certificate
		title   *string
		slug    *string
		artistN *string
		artistID *int64
	)
	if err := r.db.QueryRow(ctx, query, codeStr, locale).Scan(
		&c.ID, &c.ProductID, &c.CertCode, &c.QRKey, &c.PDFKey, &c.IssuedAt, &c.CreatedAt, &c.UpdatedAt,
		&title, &slug, &artistN, &artistID, &c.ThumbnailURL, &c.Attributes,
	); err == pgx.ErrNoRows {
		return nil, models.ErrNotFound
	} else if err != nil {
		return nil, fmt.Errorf("certificate.GetByCode: %w", err)
	}
	if title != nil {
		c.ProductTitle = *title
	}
	if slug != nil {
		c.ProductSlug = *slug
	}
	c.ArtistName = artistN
	prov, err := r.LoadProvenance(ctx, c.ID)
	if err != nil {
		return nil, fmt.Errorf("certificate.GetByCode.loadProvenance: %w", err)
	}
	c.Provenance = prov
	return &c, nil
}

func (r *Repository) GetByProductID(ctx context.Context, productID int64) (*models.Certificate, error) {
	c, err := r.scanCert(r.db.QueryRow(ctx, `SELECT`+certSelectCols+`FROM certificates c WHERE product_id=$1`, productID))
	if err == pgx.ErrNoRows {
		return nil, models.ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("certificate.GetByProductID: %w", err)
	}
	return c, nil
}

func (r *Repository) GetByID(ctx context.Context, id int64) (*models.Certificate, error) {
	c, err := r.scanCert(r.db.QueryRow(ctx, `SELECT`+certSelectCols+`FROM certificates c WHERE id=$1`, id))
	if err == pgx.ErrNoRows {
		return nil, models.ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("certificate.GetByID: %w", err)
	}
	return c, nil
}

func (r *Repository) ListAdmin(ctx context.Context, page, limit int) ([]models.Certificate, int, error) {
	var total int
	if err := r.db.QueryRow(ctx, `SELECT COUNT(*) FROM certificates`).Scan(&total); err != nil {
		return nil, 0, fmt.Errorf("certificate.ListAdmin.Count: %w", err)
	}
	if total == 0 {
		return []models.Certificate{}, 0, nil
	}
	offset := (page - 1) * limit
	rows, err := r.db.Query(ctx, `SELECT`+certSelectCols+`FROM certificates c ORDER BY issued_at DESC LIMIT $1 OFFSET $2`, limit, offset)
	if err != nil {
		return nil, 0, fmt.Errorf("certificate.ListAdmin.Query: %w", err)
	}
	defer rows.Close()
	out := []models.Certificate{}
	for rows.Next() {
		c, err := r.scanCert(rows)
		if err != nil {
			return nil, 0, fmt.Errorf("certificate.ListAdmin.Scan: %w", err)
		}
		out = append(out, *c)
	}
	return out, total, rows.Err()
}

func (r *Repository) RegenerateCode(ctx context.Context, id int64) (string, error) {
	c, err := r.NextCode(ctx)
	if err != nil {
		return "", err
	}
	tx, err := r.db.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return "", fmt.Errorf("certificate.RegenerateCode.BeginTx: %w", err)
	}
	defer tx.Rollback(ctx) //nolint:errcheck
	cmd, err := tx.Exec(ctx, `UPDATE certificates SET cert_code=$2, updated_at=NOW() WHERE id=$1`, id, c)
	if err != nil {
		return "", fmt.Errorf("certificate.RegenerateCode.Update: %w", err)
	}
	if cmd.RowsAffected() == 0 {
		return "", models.ErrNotFound
	}
	detail, _ := json.Marshal(map[string]any{"regenerated": true})
	if _, err := tx.Exec(ctx, `INSERT INTO provenance_records (certificate_id, kind, detail) VALUES ($1,'created',$2)`, id, detail); err != nil {
		return "", fmt.Errorf("certificate.RegenerateCode.Provenance: %w", err)
	}
	if err := tx.Commit(ctx); err != nil {
		return "", fmt.Errorf("certificate.RegenerateCode.Commit: %w", err)
	}
	return c, nil
}

func (r *Repository) AppendProvenance(ctx context.Context, certificateID int64, kind models.ProvenanceKind, detail json.RawMessage) error {
	if detail == nil {
		detail = json.RawMessage(`{}`)
	}
	_, err := r.db.Exec(ctx, `
		INSERT INTO provenance_records (certificate_id, kind, detail)
		VALUES ($1, $2, $3)`, certificateID, string(kind), detail)
	if err != nil {
		return fmt.Errorf("certificate.AppendProvenance: %w", err)
	}
	return nil
}

func (r *Repository) FindCertsBySKUs(ctx context.Context, skuIDs []int64) (map[int64]*models.Certificate, error) {
	out := make(map[int64]*models.Certificate)
	if len(skuIDs) == 0 {
		return out, nil
	}
	// JOIN certificates → products → skus; map sku_id → cert. All 9 selected
	// columns are scanned in one Scan (pgx requires it).
	rows, err := r.db.Query(ctx, `
		SELECT `+certSelectCols+`, s.id AS sku_id
		FROM certificates c
		JOIN products p ON p.id = c.product_id
		JOIN skus s ON s.product_id = p.id
		WHERE s.id = ANY($1)`, skuIDs)
	if err != nil {
		return nil, fmt.Errorf("certificate.FindCertsBySKUs: %w", err)
	}
	defer rows.Close()
	for rows.Next() {
		var c models.Certificate
		var skuID int64
		if err := rows.Scan(&c.ID, &c.ProductID, &c.CertCode, &c.QRKey, &c.PDFKey,
			&c.IssuedAt, &c.CreatedAt, &c.UpdatedAt, &skuID); err != nil {
			return nil, fmt.Errorf("certificate.FindCertsBySKUs.Scan: %w", err)
		}
		cc := c
		out[skuID] = &cc
	}
	return out, rows.Err()
}
