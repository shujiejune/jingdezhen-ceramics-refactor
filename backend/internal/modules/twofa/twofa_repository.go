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

	// --- Backup codes (recovery) ---
	// StoreBackupCodes deletes the user's UNUSED backup codes (keeps used for
	// audit) and inserts the given hashes. Works for both initial generation
	// (nothing to delete) and regenerate (invalidates remaining old codes).
	StoreBackupCodes(ctx context.Context, userID string, hashes []string) error
	// ConsumeBackupCode atomically marks one unused matching code as used.
	// Returns true if a code was consumed, false if no unused code matched.
	ConsumeBackupCode(ctx context.Context, userID, codeHash string) (bool, error)
	// CountUnusedBackupCodes returns how many backup codes remain (for the
	// "how many left?" UI prompt to nudge regeneration).
	CountUnusedBackupCodes(ctx context.Context, userID string) (int, error)
	// DeleteBackupCodes removes all backup codes for a user (used when 2FA is
	// disabled, so stale codes can't be revived by a later re-enrollment).
	DeleteBackupCodes(ctx context.Context, userID string) error
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

// --- Backup codes -----------------------------------------------------------

func (r *Repository) StoreBackupCodes(ctx context.Context, userID string, hashes []string) error {
	tx, err := r.db.Begin(ctx)
	if err != nil {
		return fmt.Errorf("repository.StoreBackupCodes.BeginTx: %w", err)
	}
	defer tx.Rollback(ctx) //nolint:errcheck

	// Invalidate remaining unused codes (keep used rows for audit). For initial
	// generation there are none to delete; for regenerate this clears the old set.
	if _, err := tx.Exec(ctx,
		`DELETE FROM user_2fa_backup_codes WHERE user_id = $1 AND used_at IS NULL`,
		userID); err != nil {
		return fmt.Errorf("repository.StoreBackupCodes.Delete: %w", err)
	}

	// Batch-insert the new hashed codes.
	for _, h := range hashes {
		if _, err := tx.Exec(ctx,
			`INSERT INTO user_2fa_backup_codes (user_id, code_hash) VALUES ($1, $2)`,
			userID, h); err != nil {
			return fmt.Errorf("repository.StoreBackupCodes.Insert: %w", err)
		}
	}
	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("repository.StoreBackupCodes.Commit: %w", err)
	}
	return nil
}

func (r *Repository) ConsumeBackupCode(ctx context.Context, userID, codeHash string) (bool, error) {
	// Atomic consume: only an unused matching row gets marked used. RETURNING id
	// tells us whether a row was found (no rows → no match / already used).
	var id int64
	err := r.db.QueryRow(ctx,
		`UPDATE user_2fa_backup_codes SET used_at = NOW()
		 WHERE user_id = $1 AND code_hash = $2 AND used_at IS NULL
		 RETURNING id`,
		userID, codeHash).Scan(&id)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return false, nil // no unused matching code
		}
		return false, fmt.Errorf("repository.ConsumeBackupCode: %w", err)
	}
	return true, nil
}

func (r *Repository) CountUnusedBackupCodes(ctx context.Context, userID string) (int, error) {
	var n int
	if err := r.db.QueryRow(ctx,
		`SELECT COUNT(*) FROM user_2fa_backup_codes WHERE user_id = $1 AND used_at IS NULL`,
		userID).Scan(&n); err != nil {
		return 0, fmt.Errorf("repository.CountUnusedBackupCodes: %w", err)
	}
	return n, nil
}

func (r *Repository) DeleteBackupCodes(ctx context.Context, userID string) error {
	if _, err := r.db.Exec(ctx, `DELETE FROM user_2fa_backup_codes WHERE user_id = $1`, userID); err != nil {
		return fmt.Errorf("repository.DeleteBackupCodes: %w", err)
	}
	return nil
}
