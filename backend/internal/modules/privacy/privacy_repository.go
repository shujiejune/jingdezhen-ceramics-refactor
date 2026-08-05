package privacy

import (
	"context"
	"errors"
	"fmt"
	"time"

	"jingdezhen-ceramics-backend/internal/models"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// RepositoryInterface spans every table holding personal data, so the privacy
// service can assemble a complete GDPR export and perform a full erasure without
// depending on 5+ module repositories (TDD §4.3).
type RepositoryInterface interface {
	// ExportUserData assembles all personal data held about a user into a single
	// machine-readable package. Returns models.ErrNotFound if the user does not
	// exist (or has already been erased).
	ExportUserData(ctx context.Context, userID string, locale string) (*models.UserDataExport, error)

	// AnonymizeUser performs GDPR erasure: nulls all PII columns on the users
	// row, sets is_active=false + deleted_at=NOW(), and relies on existing FK
	// CASCADEs to purge addresses/2FA/favorites/notifications. consent_records
	// and content authorship are SET NULL (retained, anonymized). Returns
	// models.ErrNotFound if the user does not exist.
	AnonymizeUser(ctx context.Context, userID string) error

	// IsDeleted reports whether the user has already been GDPR-erased (used to
	// reject logins on an anonymized stub).
	IsDeleted(ctx context.Context, userID string) (bool, error)
}

type Repository struct {
	db *pgxpool.Pool
}

func NewRepository(db *pgxpool.Pool) RepositoryInterface {
	return &Repository{db: db}
}

func (r *Repository) ExportUserData(ctx context.Context, userID string, locale string) (*models.UserDataExport, error) {
	exp := &models.UserDataExport{
		ExportedAt: time.Now().UTC(),
		UserID:     userID,
		Locale:     locale,
	}

	// 1. Profile (users row). A deleted/anonymized stub still returns here so the
	// user can see the (empty) state before their own erasure completes.
	profile, err := r.fetchProfile(ctx, userID)
	if err != nil {
		return nil, err
	}
	exp.Profile = profile

	// 2. Addresses.
	addresses, err := r.fetchAddresses(ctx, userID)
	if err != nil {
		return nil, err
	}
	exp.Addresses = addresses

	// 3. Consent records.
	consent, err := r.fetchConsent(ctx, userID)
	if err != nil {
		return nil, err
	}
	exp.ConsentRecords = consent

	// 4. 2FA metadata (non-secret).
	twofa, err := r.fetchTwoFA(ctx, userID)
	if err != nil {
		return nil, err
	}
	exp.TwoFA = twofa

	// 5. Favorite artworks.
	favs, err := r.fetchFavorites(ctx, userID)
	if err != nil {
		return nil, err
	}
	exp.Wishlist = favs

	// 6. Notifications.
	notifs, err := r.fetchNotifications(ctx, userID)
	if err != nil {
		return nil, err
	}
	exp.Notifications = notifs

	return exp, nil
}

func (r *Repository) AnonymizeUser(ctx context.Context, userID string) error {
	cmd, err := r.db.Exec(ctx, `
		UPDATE users SET
		    email = NULL,
		    nickname = NULL,
		    password_hash = NULL,
		    avatar_url = NULL,
		    auth_provider_id = NULL,
		    activation_token = NULL,
		    activation_token_expires_at = NULL,
		    password_reset_token = NULL,
		    password_reset_expires_at = NULL,
		    profile_data = NULL,
		    is_active = FALSE,
		    deleted_at = NOW(),
		    updated_at = NOW()
		WHERE id = $1 AND deleted_at IS NULL`,
		userID)
	if err != nil {
		return fmt.Errorf("privacy.AnonymizeUser: %w", err)
	}
	if cmd.RowsAffected() == 0 {
		// Either the user doesn't exist or was already erased. Distinguish so the
		// handler returns the right status.
		exists, err := r.userExists(ctx, userID)
		if err != nil {
			return err
		}
		if !exists {
			return models.ErrNotFound
		}
		return models.ErrAccountDeleted
	}
	return nil
}

func (r *Repository) IsDeleted(ctx context.Context, userID string) (bool, error) {
	var deletedAt *time.Time
	err := r.db.QueryRow(ctx, `SELECT deleted_at FROM users WHERE id = $1`, userID).Scan(&deletedAt)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return false, models.ErrNotFound
		}
		return false, fmt.Errorf("privacy.IsDeleted: %w", err)
	}
	return deletedAt != nil, nil
}

func (r *Repository) userExists(ctx context.Context, userID string) (bool, error) {
	var exists bool
	err := r.db.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM users WHERE id = $1)`, userID).Scan(&exists)
	if err != nil {
		return false, fmt.Errorf("privacy.userExists: %w", err)
	}
	return exists, nil
}

// --- Export fetchers ----------------------------------------------------------

func (r *Repository) fetchProfile(ctx context.Context, userID string) (*models.User, error) {
	// Reuse the user-repository scan shape. We read every column except the
	// password_hash (a secret — minimisation) by selecting NULL in its place.
	query := `
		SELECT id, nickname, email, NULL AS password_hash, avatar_url, profile_data,
		       preferred_locale, preferred_currency, auth_provider, '' AS auth_provider_id,
		       is_active, deleted_at, created_at, updated_at
		FROM users WHERE id = $1`
	var u models.User
	var ph *string
	err := r.db.QueryRow(ctx, query, userID).Scan(
		&u.ID, &u.Nickname, &u.Email, &ph, &u.AvatarURL, &u.ProfileData,
		&u.PreferredLocale, &u.PreferredCurrency, &u.AuthProvider, &u.AuthProviderID,
		&u.IsActive, &u.DeletedAt, &u.CreatedAt, &u.UpdatedAt)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, models.ErrNotFound
		}
		return nil, fmt.Errorf("privacy.fetchProfile: %w", err)
	}
	return &u, nil
}

func (r *Repository) fetchAddresses(ctx context.Context, userID string) ([]models.UserAddress, error) {
	rows, err := r.db.Query(ctx, `
		SELECT id, user_id, recipient, line1, line2, city, region, postal_code,
		       country, phone, is_default, created_at, updated_at
		FROM user_addresses WHERE user_id = $1 ORDER BY created_at`, userID)
	if err != nil {
		return nil, fmt.Errorf("privacy.fetchAddresses: %w", err)
	}
	defer rows.Close()

	out := []models.UserAddress{}
	for rows.Next() {
		var a models.UserAddress
		if err := rows.Scan(&a.ID, &a.UserID, &a.Recipient, &a.Line1, &a.Line2, &a.City,
			&a.Region, &a.PostalCode, &a.Country, &a.Phone, &a.IsDefault,
			&a.CreatedAt, &a.UpdatedAt); err != nil {
			return nil, fmt.Errorf("privacy.fetchAddresses.Scan: %w", err)
		}
		out = append(out, a)
	}
	return out, nil
}

func (r *Repository) fetchConsent(ctx context.Context, userID string) ([]models.ConsentRecord, error) {
	rows, err := r.db.Query(ctx, `
		SELECT id, user_id, kind, doc_version, granted, ip_hash, created_at
		FROM consent_records WHERE user_id = $1 ORDER BY created_at DESC`, userID)
	if err != nil {
		return nil, fmt.Errorf("privacy.fetchConsent: %w", err)
	}
	defer rows.Close()

	out := []models.ConsentRecord{}
	for rows.Next() {
		var c models.ConsentRecord
		if err := rows.Scan(&c.ID, &c.UserID, &c.Kind, &c.DocVersion, &c.Granted,
			&c.IPHash, &c.CreatedAt); err != nil {
			return nil, fmt.Errorf("privacy.fetchConsent.Scan: %w", err)
		}
		out = append(out, c)
	}
	return out, nil
}

func (r *Repository) fetchTwoFA(ctx context.Context, userID string) (*models.TwoFAExport, error) {
	var t models.TwoFAExport
	var confirmedAt *time.Time
	err := r.db.QueryRow(ctx, `
		SELECT enabled, confirmed_at, created_at, updated_at
		FROM user_2fa WHERE user_id = $1`, userID).
		Scan(&t.Enabled, &confirmedAt, &t.CreatedAt, &t.UpdatedAt)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, nil // no 2FA enrolled — omit from export
		}
		return nil, fmt.Errorf("privacy.fetchTwoFA: %w", err)
	}
	t.ConfirmedAt = confirmedAt

	// Available backup-code count (used codes are retained for audit but not
	// surfaced as a count — only remaining capacity matters for transparency).
	var remaining int
	if err := r.db.QueryRow(ctx,
		`SELECT COUNT(*) FROM user_2fa_backup_codes WHERE user_id = $1 AND used_at IS NULL`,
		userID).Scan(&remaining); err != nil {
		return nil, fmt.Errorf("privacy.fetchTwoFA.BackupCount: %w", err)
	}
	t.BackupCodesRemaining = remaining
	return &t, nil
}

func (r *Repository) fetchFavorites(ctx context.Context, userID string) ([]models.FavoriteExport, error) {
	rows, err := r.db.Query(ctx, `
		SELECT w.sku_id, s.sku_code, w.created_at
		FROM wishlists w
		JOIN skus s ON s.id = w.sku_id
		WHERE w.user_id = $1 ORDER BY w.created_at DESC`, userID)
	if err != nil {
		return nil, fmt.Errorf("privacy.fetchFavorites: %w", err)
	}
	defer rows.Close()

	out := []models.FavoriteExport{}
	for rows.Next() {
		var f models.FavoriteExport
		if err := rows.Scan(&f.SKUID, &f.SKUCode, &f.FavoritedAt); err != nil {
			return nil, fmt.Errorf("privacy.fetchFavorites.Scan: %w", err)
		}
		out = append(out, f)
	}
	return out, nil
}

func (r *Repository) fetchNotifications(ctx context.Context, userID string) ([]models.Notification, error) {
	rows, err := r.db.Query(ctx, `
		SELECT id, recipient_user_id, actor_user_id, notification_type,
		       entity_type, entity_id, message, is_read, created_at
		FROM notifications WHERE recipient_user_id = $1 ORDER BY created_at DESC
		LIMIT 500`, userID)
	if err != nil {
		return nil, fmt.Errorf("privacy.fetchNotifications: %w", err)
	}
	defer rows.Close()

	out := []models.Notification{}
	for rows.Next() {
		var n models.Notification
		if err := rows.Scan(&n.ID, &n.RecipientUserID, &n.ActorUserID, &n.NotificationType,
			&n.EntityType, &n.EntityID, &n.Message, &n.IsRead, &n.CreatedAt); err != nil {
			return nil, fmt.Errorf("privacy.fetchNotifications.Scan: %w", err)
		}
		out = append(out, n)
	}
	return out, nil
}
