package twofa

import (
	"context"
	"errors"
	"fmt"
	"jingdezhen-ceramics-backend/internal/models"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// RepositoryInterface defines TOTP-secret storage operations.
type RepositoryInterface interface {
	// Get loads a user's 2FA record. Returns ErrNotFound if none exists.
	Get(ctx context.Context, userID string) (*models.TwoFARecord, error)
	// UpsertStage inserts or replaces the encrypted TOTP secret with enabled=FALSE
	// (unconfirmed). Used during enrollment; the user must confirm to enable.
	UpsertStage(ctx context.Context, userID string, encSecret []byte) error
	// Confirm marks the staged enrollment as enabled+confirmed.
	Confirm(ctx context.Context, userID string) error
	// Disable sets enabled=FALSE (keeps the secret; re-enrollment can re-stage).
	Disable(ctx context.Context, userID string) error
	// Delete removes the 2FA record entirely (used when a super_admin is
	// downgraded or the user resets 2FA).
	Delete(ctx context.Context, userID string) error
}

type Repository struct {
	db *pgxpool.Pool
}

func NewRepository(db *pgxpool.Pool) RepositoryInterface {
	return &Repository{db: db}
}

const twoFAColumns = "user_id, totp_secret_enc, enabled, confirmed_at, created_at, updated_at"

func (r *Repository) scanRecord(row pgx.Row) (*models.TwoFARecord, error) {
	var rec models.TwoFARecord
	err := row.Scan(&rec.UserID, &rec.EncryptedSecret, &rec.Enabled, &rec.ConfirmedAt, &rec.CreatedAt, &rec.UpdatedAt)
	if err != nil {
		return nil, err
	}
	return &rec, nil
}

func (r *Repository) Get(ctx context.Context, userID string) (*models.TwoFARecord, error) {
	query := `SELECT ` + twoFAColumns + ` FROM user_2fa WHERE user_id = $1`
	rec, err := r.scanRecord(r.db.QueryRow(ctx, query, userID))
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, models.ErrNotFound
		}
		return nil, fmt.Errorf("repository.TwoFA.Get: %w", err)
	}
	return rec, nil
}

func (r *Repository) UpsertStage(ctx context.Context, userID string, encSecret []byte) error {
	query := `
		INSERT INTO user_2fa (user_id, totp_secret_enc, enabled, confirmed_at)
		VALUES ($1, $2, FALSE, NULL)
		ON CONFLICT (user_id) DO UPDATE
			SET totp_secret_enc = EXCLUDED.totp_secret_enc,
			    enabled = FALSE,
			    confirmed_at = NULL,
			    updated_at = NOW()`
	_, err := r.db.Exec(ctx, query, userID, encSecret)
	if err != nil {
		return fmt.Errorf("repository.TwoFA.UpsertStage: %w", err)
	}
	return nil
}

func (r *Repository) Confirm(ctx context.Context, userID string) error {
	cmd, err := r.db.Exec(ctx,
		`UPDATE user_2fa SET enabled = TRUE, confirmed_at = NOW(), updated_at = NOW() WHERE user_id = $1`,
		userID)
	if err != nil {
		return fmt.Errorf("repository.TwoFA.Confirm: %w", err)
	}
	if cmd.RowsAffected() == 0 {
		return models.ErrNotFound
	}
	return nil
}

func (r *Repository) Disable(ctx context.Context, userID string) error {
	cmd, err := r.db.Exec(ctx,
		`UPDATE user_2fa SET enabled = FALSE, updated_at = NOW() WHERE user_id = $1`,
		userID)
	if err != nil {
		return fmt.Errorf("repository.TwoFA.Disable: %w", err)
	}
	if cmd.RowsAffected() == 0 {
		return models.ErrNotFound
	}
	return nil
}

func (r *Repository) Delete(ctx context.Context, userID string) error {
	cmd, err := r.db.Exec(ctx, `DELETE FROM user_2fa WHERE user_id = $1`, userID)
	if err != nil {
		return fmt.Errorf("repository.TwoFA.Delete: %w", err)
	}
	if cmd.RowsAffected() == 0 {
		return models.ErrNotFound
	}
	return nil
}
