package user

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"jingdezhen-ceramics-backend/internal/models"
	"log"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
)

// RepositoryInterface defines methods for interacting with user storage.
type RepositoryInterface interface {
	FindByID(ctx context.Context, userID string) (*models.User, error)
	FindByIDs(ctx context.Context, userIDs []string) ([]models.User, error)
	FindByEmail(ctx context.Context, email string) (*models.User, error)
	FindByNickname(ctx context.Context, nickname string) (*models.User, error)
	FindByPasswordResetToken(ctx context.Context, token string) (*models.User, error)

	SetPasswordResetToken(ctx context.Context, userID string, token string, expiresAt time.Time) error
	UpdatePasswordAndClearResetToken(ctx context.Context, userID string, passwordHash string) error
	UpdateActivationToken(ctx context.Context, userID, newToken string, expiresAt time.Time) error

	CreateInactiveUser(ctx context.Context, user *models.User, passwordHash, activationToken string, expiresAt time.Time) (*models.User, error)
	ActivateUser(ctx context.Context, token string) (*models.User, error)
	CreateOAuthUser(ctx context.Context, user *models.User) (*models.User, error)
	Update(ctx context.Context, userID string, updateData models.UserUpdateData) (*models.User, error)

	ListAll(ctx context.Context, page, limit int) ([]models.User, int, error) // Admin: list users
	ListByRole(ctx context.Context, roleKey string) ([]models.User, error)        // staff users with a role (low-stock alert recipients)
	GetUserRoles(ctx context.Context, userID string) ([]string, error)        // RBAC: load role keys
	AssignRole(ctx context.Context, userID string, roleKey string) error     // Admin: set single staff role
}

// DBExecutor represents anything that can execute a SQL query
// (a connection pool or a transaction).
type DBExecutor interface {
	Exec(ctx context.Context, sql string, arguments ...any) (pgconn.CommandTag, error)
	Query(ctx context.Context, sql string, args ...any) (pgx.Rows, error)
	QueryRow(ctx context.Context, sql string, args ...any) pgx.Row
}

type Repository struct {
	db       *pgxpool.Pool
	executor DBExecutor
}

func NewRepository(db *pgxpool.Pool) RepositoryInterface {
	return &Repository{
		db:       db,
		executor: db,
	}
}

// BeginTx starts a new database transaction.
func (r *Repository) BeginTx(ctx context.Context) (pgx.Tx, error) {
	return r.db.Begin(ctx)
}

// WithTx returns a repository scoped to the provided transaction.
func (r *Repository) WithTx(tx pgx.Tx) *Repository {
	return &Repository{
		db:       r.db,
		executor: tx,
	}
}

// Columns selected for a user without the password hash. Note: roles are loaded
// separately via GetUserRoles (PRD §3.4.1 RBAC).
const userColumns = "id, nickname, email, avatar_url, profile_data, preferred_locale, preferred_currency, auth_provider, is_active, deleted_at, created_at, updated_at"

// Columns selected for a user including the password hash (login flows).
const userColumnsWithPassword = "id, nickname, email, password_hash, avatar_url, profile_data, preferred_locale, preferred_currency, auth_provider, is_active, deleted_at, created_at, updated_at"

func (r *Repository) scanUser(row pgx.Row) (*models.User, error) {
	var user models.User
	var avatarURL sql.NullString

	err := row.Scan(
		&user.ID,
		&user.Nickname,
		&user.Email,
		&avatarURL,
		&user.ProfileData,
		&user.PreferredLocale,
		&user.PreferredCurrency,
		&user.AuthProvider,
		&user.IsActive,
		&user.DeletedAt,
		&user.CreatedAt,
		&user.UpdatedAt,
	)
	if err != nil {
		return nil, err
	}
	if avatarURL.Valid {
		user.AvatarURL = &avatarURL.String
	} else {
		user.AvatarURL = nil
	}

	return &user, nil
}

func (r *Repository) scanUserWithPasswordHash(row pgx.Row) (*models.User, error) {
	var user models.User
	var passwordHash sql.NullString
	var avatarURL sql.NullString

	err := row.Scan(
		&user.ID,
		&user.Nickname,
		&user.Email,
		&passwordHash,
		&avatarURL,
		&user.ProfileData,
		&user.PreferredLocale,
		&user.PreferredCurrency,
		&user.AuthProvider,
		&user.IsActive,
		&user.DeletedAt,
		&user.CreatedAt,
		&user.UpdatedAt,
	)
	if err != nil {
		return nil, err
	}

	if passwordHash.Valid {
		user.PasswordHash = &passwordHash.String
	} else {
		user.PasswordHash = nil
	}
	if avatarURL.Valid {
		user.AvatarURL = &avatarURL.String
	} else {
		user.AvatarURL = nil
	}

	return &user, nil
}

func (r *Repository) FindByID(ctx context.Context, userID string) (*models.User, error) {
	query := `SELECT ` + userColumns + ` FROM users WHERE id = $1`
	row := r.executor.QueryRow(ctx, query, userID)
	user, err := r.scanUser(row)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, models.ErrNotFound
		}
		return nil, fmt.Errorf("repository.FindByID: %w", err)
	}
	return user, nil
}

// Used in the notification service to avoid the N+1 query problem.
// NOTE: SELECT * here relies on struct tags; the legacy role column is gone,
// so pgx.RowToStructByName maps the remaining columns. Roles are not loaded.
func (r *Repository) FindByIDs(ctx context.Context, userIDs []string) ([]models.User, error) {
	if len(userIDs) == 0 {
		return []models.User{}, nil
	}
	query := `SELECT ` + userColumns + ` FROM users WHERE id = ANY($1)`
	rows, err := r.db.Query(ctx, query, userIDs)
	if err != nil {
		return nil, fmt.Errorf("query for users by ids failed: %w", err)
	}
	users, err := pgx.CollectRows(rows, pgx.RowToStructByName[models.User])
	if err != nil {
		return nil, fmt.Errorf("failed to collect user rows: %w", err)
	}
	return users, nil
}

func (r *Repository) FindByEmail(ctx context.Context, email string) (*models.User, error) {
	query := `SELECT ` + userColumnsWithPassword + ` FROM users WHERE email = $1`
	row := r.executor.QueryRow(ctx, query, email)
	user, err := r.scanUserWithPasswordHash(row)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, models.ErrNotFound
		}
		return nil, fmt.Errorf("repository.FindByEmail: %w", err)
	}
	return user, nil
}

func (r *Repository) FindByNickname(ctx context.Context, nickname string) (*models.User, error) {
	query := `SELECT ` + userColumns + ` FROM users WHERE nickname = $1`
	row := r.executor.QueryRow(ctx, query, nickname)
	user, err := r.scanUser(row)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, models.ErrNotFound
		}
		return nil, fmt.Errorf("repository.FindByNickname: %w", err)
	}
	return user, nil
}

func (r *Repository) FindByPasswordResetToken(ctx context.Context, token string) (*models.User, error) {
	query := `
	SELECT ` + userColumnsWithPassword + `, auth_provider_id
	FROM users
	WHERE password_reset_token = $1 AND password_reset_expires_at > NOW()
	`
	// auth_provider_id is nullable; scan it directly into the pointer field.
	row := r.executor.QueryRow(ctx, query, token)
	var user models.User
	var passwordHash, avatarURL sql.NullString
	err := row.Scan(
		&user.ID, &user.Nickname, &user.Email, &passwordHash, &avatarURL,
		&user.ProfileData, &user.PreferredLocale, &user.PreferredCurrency,
		&user.AuthProvider, &user.IsActive,
		&user.CreatedAt, &user.UpdatedAt, &user.AuthProviderID,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, models.ErrInvalidToken
		}
		return nil, fmt.Errorf("repository.FindUserByPasswordResetToken: %w", err)
	}
	if passwordHash.Valid {
		user.PasswordHash = &passwordHash.String
	}
	if avatarURL.Valid {
		user.AvatarURL = &avatarURL.String
	}
	return &user, nil
}

func (r *Repository) SetPasswordResetToken(ctx context.Context, userID string, token string, expiresAt time.Time) error {
	log.Printf("DATABASE: Saving reset token [%s] for user [%s]", token, userID)
	query := `
	UPDATE users
	SET password_reset_token = $1, password_reset_expires_at = $2, updated_at = NOW()
	WHERE id = $3
	`
	cmdTag, err := r.db.Exec(ctx, query, token, expiresAt, userID)
	if err != nil {
		return fmt.Errorf("repository.SetPasswordResetToken: %w", err)
	}
	if cmdTag.RowsAffected() == 0 {
		return models.ErrNotFound
	}
	return nil
}

func (r *Repository) UpdatePasswordAndClearResetToken(ctx context.Context, userID string, passwordHash string) error {
	query := `
	UPDATE users
	SET password_hash = $1, password_reset_token = $2, updated_at = NOW()
	WHERE id = $3
	`
	cmdTag, err := r.db.Exec(ctx, query, passwordHash, "", userID)
	if err != nil {
		return fmt.Errorf("repository.UpdatePasswordAndClearResetToken: %w", err)
	}
	if cmdTag.RowsAffected() == 0 {
		return models.ErrNotFound
	}
	return nil
}

func (r *Repository) UpdateActivationToken(ctx context.Context, userID, newToken string, expiresAt time.Time) error {
	query := `
	UPDATE users
	SET activation_token = $1, activation_token_expires_at = $2, updated_at = NOW()
	WHERE id = $3
	`
	cmdTag, err := r.db.Exec(ctx, query, newToken, expiresAt, userID)
	if err != nil {
		return fmt.Errorf("repository.UpdateActivationToken: %w", err)
	}
	if cmdTag.RowsAffected() == 0 {
		return models.ErrNotFound
	}
	return nil
}

// CreateInactiveUser is used by the email/password signup flow.
// New users are customers by default (no user_roles row; PRD §3.4.1).
func (r *Repository) CreateInactiveUser(ctx context.Context, user *models.User, passwordHash, activationToken string, expiresAt time.Time) (*models.User, error) {
	query := `
        INSERT INTO users (nickname, email, password_hash, activation_token, activation_token_expires_at, auth_provider)
        VALUES ($1, $2, $3, $4, $5, 'email')
        RETURNING id, is_active, auth_provider, created_at, updated_at`
	err := r.db.QueryRow(ctx, query,
		user.Nickname, user.Email, passwordHash, activationToken, expiresAt,
	).Scan(&user.ID, &user.IsActive, &user.AuthProvider, &user.CreatedAt, &user.UpdatedAt)
	if err != nil {
		return nil, fmt.Errorf("repository.CreateInactiveUser: %w", err)
	}
	return user, err
}

func (r *Repository) ActivateUser(ctx context.Context, token string) (*models.User, error) {
	user := &models.User{}
	query := `
        UPDATE users
        SET is_active = TRUE, activation_token = NULL, activation_token_expires_at = NULL, updated_at = NOW()
        WHERE activation_token = $1 AND activation_token_expires_at > NOW() AND is_active = FALSE
        RETURNING ` + userColumns

	row := r.executor.QueryRow(ctx, query, token)
	user, err := r.scanUser(row)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, models.ErrInvalidToken
		}
		return nil, fmt.Errorf("repository.ActivateUser: %w", err)
	}
	return user, nil
}

// CreateOAuthUser is used by the OAuth signup flow (Google / WhatsApp).
// OAuth users are customers by default (no staff role assigned).
func (r *Repository) CreateOAuthUser(ctx context.Context, user *models.User) (*models.User, error) {
	query := `
        INSERT INTO users (nickname, email, auth_provider, auth_provider_id, is_active)
        VALUES ($1, $2, $3, $4, TRUE)
        RETURNING id, created_at, updated_at`
	err := r.db.QueryRow(ctx, query,
		user.Nickname, user.Email, user.AuthProvider, user.AuthProviderID,
	).Scan(&user.ID, &user.CreatedAt, &user.UpdatedAt)
	if err != nil {
		return nil, fmt.Errorf("repository.CreateOAuthUser: %w", err)
	}
	return user, nil
}

func (r *Repository) Update(ctx context.Context, userID string, data models.UserUpdateData) (*models.User, error) {
	var setClauses []string
	var args []interface{}
	argIdx := 1

	if data.Nickname != nil {
		setClauses = append(setClauses, fmt.Sprintf("nickname = $%d", argIdx))
		args = append(args, *data.Nickname)
		argIdx++
	}
	if data.AvatarURL != nil {
		setClauses = append(setClauses, fmt.Sprintf("avatar_url = $%d", argIdx))
		args = append(args, *data.AvatarURL)
		argIdx++
	}
	if data.OtherContact != nil {
		setClauses = append(setClauses, fmt.Sprintf("profile_data = jsonb_set(COALESCE(profile_data, '{}'::jsonb), '{other_contact}', $%d::jsonb)", argIdx))
		args = append(args, *data.OtherContact)
		argIdx++
	}
	if data.PreferredLocale != nil {
		setClauses = append(setClauses, fmt.Sprintf("preferred_locale = $%d", argIdx))
		args = append(args, *data.PreferredLocale)
		argIdx++
	}
	if data.PreferredCurrency != nil {
		setClauses = append(setClauses, fmt.Sprintf("preferred_currency = $%d", argIdx))
		args = append(args, *data.PreferredCurrency)
		argIdx++
	}

	if len(setClauses) == 0 {
		return r.FindByID(ctx, userID)
	}

	setClauses = append(setClauses, fmt.Sprintf("updated_at = $%d", argIdx))
	args = append(args, time.Now())
	argIdx++
	args = append(args, userID)

	query := fmt.Sprintf(`UPDATE users SET %s WHERE id = $%d RETURNING %s`,
		strings.Join(setClauses, ", "), argIdx, userColumns)

	row := r.executor.QueryRow(ctx, query, args...)
	updatedUser, err := r.scanUser(row)
	if err != nil {
		return nil, fmt.Errorf("repository.UpdateUser: %w", err)
	}
	return updatedUser, nil
}

// --- Admin / RBAC methods ------------------------------------------------------

func (r *Repository) ListAll(ctx context.Context, page, limit int) ([]models.User, int, error) {
	offset := (page - 1) * limit
	query := `SELECT ` + userColumnsWithPassword + `, auth_provider_id FROM users ORDER BY created_at DESC LIMIT $1 OFFSET $2`
	rows, err := r.db.Query(ctx, query, limit, offset)
	if err != nil {
		return nil, 0, fmt.Errorf("repository.ListAllUsers: %w", err)
	}
	defer rows.Close()

	users := []models.User{}
	for rows.Next() {
		var user models.User
		var passwordHash, avatarURL sql.NullString
		if err := rows.Scan(
			&user.ID, &user.Nickname, &user.Email, &passwordHash, &avatarURL,
			&user.ProfileData, &user.PreferredLocale, &user.PreferredCurrency,
			&user.AuthProvider, &user.IsActive,
			&user.CreatedAt, &user.UpdatedAt, &user.AuthProviderID,
		); err != nil {
			return nil, 0, fmt.Errorf("repository.ListAllUsers.Scan: %w", err)
		}
		if passwordHash.Valid {
			user.PasswordHash = &passwordHash.String
		}
		if avatarURL.Valid {
			user.AvatarURL = &avatarURL.String
		}
		users = append(users, user)
	}

	var total int
	if err := r.db.QueryRow(ctx, "SELECT COUNT(*) FROM users").Scan(&total); err != nil {
		return nil, 0, fmt.Errorf("repository.ListAllUsers.Count: %w", err)
	}
	return users, total, nil
}

// ListByRole returns active users holding a given staff role (e.g. all
// ecommerce_operator users for low-stock alert delivery, PRD §3.4.1).
func (r *Repository) ListByRole(ctx context.Context, roleKey string) ([]models.User, error) {
	// Qualify every column with u. — the JOIN to roles (which also has id) would
	// otherwise make "id" ambiguous (userColumns is unqualified for the
	// no-JOIN queries).
	query := `SELECT u.id, u.nickname, u.email, u.avatar_url, u.profile_data,
	                 u.preferred_locale, u.preferred_currency, u.auth_provider,
	                 u.is_active, u.deleted_at, u.created_at, u.updated_at
		FROM users u
		JOIN user_roles ur ON ur.user_id = u.id
		JOIN roles r ON r.id = ur.role_id
		WHERE r.key = $1 AND u.is_active = TRUE AND u.deleted_at IS NULL
		ORDER BY u.created_at ASC`
	rows, err := r.db.Query(ctx, query, roleKey)
	if err != nil {
		return nil, fmt.Errorf("repository.ListByRole: %w", err)
	}
	defer rows.Close()
	users := []models.User{}
	for rows.Next() {
		user, err := r.scanUser(rows)
		if err != nil {
			return nil, fmt.Errorf("repository.ListByRole.Scan: %w", err)
		}
		users = append(users, *user)
	}
	return users, rows.Err()
}

// GetUserRoles returns the staff role keys assigned to a user (empty for customers).
func (r *Repository) GetUserRoles(ctx context.Context, userID string) ([]string, error) {
	query := `
	SELECT r.key
	FROM user_roles ur
	JOIN roles r ON r.id = ur.role_id
	WHERE ur.user_id = $1
	ORDER BY r.key`
	rows, err := r.executor.Query(ctx, query, userID)
	if err != nil {
		return nil, fmt.Errorf("repository.GetUserRoles: %w", err)
	}
	defer rows.Close()

	var roles []string
	for rows.Next() {
		var key string
		if err := rows.Scan(&key); err != nil {
			return nil, fmt.Errorf("repository.GetUserRoles.Scan: %w", err)
		}
		roles = append(roles, key)
	}
	if roles == nil {
		roles = []string{}
	}
	return roles, nil
}

// AssignRole replaces the user's staff role assignment with a single role.
// In v1 a user holds at most one staff role (PRD §3.4.1 — no custom roles).
// Returns models.ErrNotFound if roleKey is not a seeded staff role.
func (r *Repository) AssignRole(ctx context.Context, userID string, roleKey string) error {
	tx, err := r.db.Begin(ctx)
	if err != nil {
		return fmt.Errorf("repository.AssignRole.BeginTx: %w", err)
	}
	defer tx.Rollback(ctx) //nolint:errcheck // committed below on success

	// Clear any existing role assignments for this user.
	if _, err := tx.Exec(ctx, `DELETE FROM user_roles WHERE user_id = $1`, userID); err != nil {
		return fmt.Errorf("repository.AssignRole.Delete: %w", err)
	}

	// Insert the new role (no-op row count if roleKey is invalid).
	cmd, err := tx.Exec(ctx, `
		INSERT INTO user_roles (user_id, role_id)
		SELECT $1, id FROM roles WHERE key = $2`, userID, roleKey)
	if err != nil {
		return fmt.Errorf("repository.AssignRole.Insert: %w", err)
	}
	if cmd.RowsAffected() == 0 {
		return fmt.Errorf("repository.AssignRole: unknown role key %q: %w", roleKey, models.ErrNotFound)
	}

	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("repository.AssignRole.Commit: %w", err)
	}
	return nil
}
