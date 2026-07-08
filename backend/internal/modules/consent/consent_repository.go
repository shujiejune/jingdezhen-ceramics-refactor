package consent

import (
	"context"
	"errors"
	"fmt"
	"jingdezhen-ceramics-backend/internal/models"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// RepositoryInterface defines consent-ledger storage operations.
type RepositoryInterface interface {
	Record(ctx context.Context, r models.ConsentRecord) (*models.ConsentRecord, error)
	// LatestForUser returns the most recent consent record for a given user +
	// kind, or ErrNotFound if none exists. Used to check current consent state.
	LatestForUser(ctx context.Context, userID string, kind models.ConsentKind) (*models.ConsentRecord, error)
	// LatestForIPHash returns the most recent anonymous consent record by IP hash.
	LatestForIPHash(ctx context.Context, ipHash string, kind models.ConsentKind) (*models.ConsentRecord, error)
	// ListForUser returns all consent records for a user, newest first (GDPR
	// data-export / audit trail).
	ListForUser(ctx context.Context, userID string) ([]models.ConsentRecord, error)
}

type Repository struct {
	db *pgxpool.Pool
}

func NewRepository(db *pgxpool.Pool) RepositoryInterface {
	return &Repository{db: db}
}

const consentColumns = "id, user_id, kind, doc_version, granted, ip_hash, created_at"

func (r *Repository) scanRecord(row pgx.Row) (*models.ConsentRecord, error) {
	var rec models.ConsentRecord
	var userID, ipHash *string // nullable
	err := row.Scan(&rec.ID, &userID, &rec.Kind, &rec.DocVersion, &rec.Granted, &ipHash, &rec.CreatedAt)
	if err != nil {
		return nil, err
	}
	rec.UserID = userID
	rec.IPHash = ipHash
	return &rec, nil
}

func (r *Repository) Record(ctx context.Context, rec models.ConsentRecord) (*models.ConsentRecord, error) {
	query := `
		INSERT INTO consent_records (user_id, kind, doc_version, granted, ip_hash)
		VALUES ($1, $2, $3, $4, $5)
		RETURNING ` + consentColumns
	row := r.db.QueryRow(ctx, query, rec.UserID, rec.Kind, rec.DocVersion, rec.Granted, rec.IPHash)
	out, err := r.scanRecord(row)
	if err != nil {
		return nil, fmt.Errorf("repository.RecordConsent: %w", err)
	}
	return out, nil
}

func (r *Repository) LatestForUser(ctx context.Context, userID string, kind models.ConsentKind) (*models.ConsentRecord, error) {
	query := `
		SELECT ` + consentColumns + ` FROM consent_records
		WHERE user_id = $1 AND kind = $2
		ORDER BY created_at DESC LIMIT 1`
	row := r.db.QueryRow(ctx, query, userID, kind)
	rec, err := r.scanRecord(row)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, models.ErrNotFound
		}
		return nil, fmt.Errorf("repository.LatestConsentForUser: %w", err)
	}
	return rec, nil
}

func (r *Repository) LatestForIPHash(ctx context.Context, ipHash string, kind models.ConsentKind) (*models.ConsentRecord, error) {
	query := `
		SELECT ` + consentColumns + ` FROM consent_records
		WHERE ip_hash = $1 AND kind = $2 AND user_id IS NULL
		ORDER BY created_at DESC LIMIT 1`
	row := r.db.QueryRow(ctx, query, ipHash, kind)
	rec, err := r.scanRecord(row)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, models.ErrNotFound
		}
		return nil, fmt.Errorf("repository.LatestConsentForIP: %w", err)
	}
	return rec, nil
}

func (r *Repository) ListForUser(ctx context.Context, userID string) ([]models.ConsentRecord, error) {
	query := `
		SELECT ` + consentColumns + ` FROM consent_records
		WHERE user_id = $1
		ORDER BY created_at DESC`
	rows, err := r.db.Query(ctx, query, userID)
	if err != nil {
		return nil, fmt.Errorf("repository.ListConsentForUser: %w", err)
	}
	defer rows.Close()

	out := []models.ConsentRecord{}
	for rows.Next() {
		rec, err := r.scanRecord(rows)
		if err != nil {
			return nil, fmt.Errorf("repository.ListConsentForUser.Scan: %w", err)
		}
		out = append(out, *rec)
	}
	return out, nil
}
