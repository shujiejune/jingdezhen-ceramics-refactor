package address

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	"jingdezhen-ceramics-backend/internal/models"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// RepositoryInterface defines user-address storage operations.
// Every method is scoped to userID so callers cannot read or mutate
// another user's addresses (ownership enforced at the service layer too).
type RepositoryInterface interface {
	ListByUser(ctx context.Context, userID string) ([]models.UserAddress, error)
	GetByID(ctx context.Context, userID string, id int64) (*models.UserAddress, error)
	Create(ctx context.Context, userID string, req models.CreateAddressRequest) (*models.UserAddress, error)
	Update(ctx context.Context, userID string, id int64, req models.UpdateAddressRequest) (*models.UserAddress, error)
	Delete(ctx context.Context, userID string, id int64) error
	SetDefault(ctx context.Context, userID string, id int64) error
}

type Repository struct {
	db *pgxpool.Pool
}

func NewRepository(db *pgxpool.Pool) RepositoryInterface {
	return &Repository{db: db}
}

const addressColumns = "id, user_id, recipient, line1, line2, city, region, postal_code, country, phone, is_default, created_at, updated_at"

func (r *Repository) scanAddress(row pgx.Row) (*models.UserAddress, error) {
	var a models.UserAddress
	var line2, region, postalCode, phone sql.NullString
	err := row.Scan(
		&a.ID, &a.UserID, &a.Recipient, &a.Line1, &line2, &a.City, &region,
		&postalCode, &a.Country, &phone, &a.IsDefault, &a.CreatedAt, &a.UpdatedAt,
	)
	if err != nil {
		return nil, err
	}
	if line2.Valid {
		a.Line2 = &line2.String
	}
	if region.Valid {
		a.Region = &region.String
	}
	if postalCode.Valid {
		a.PostalCode = &postalCode.String
	}
	if phone.Valid {
		a.Phone = &phone.String
	}
	return &a, nil
}

func (r *Repository) ListByUser(ctx context.Context, userID string) ([]models.UserAddress, error) {
	query := `SELECT ` + addressColumns + ` FROM user_addresses WHERE user_id = $1 ORDER BY is_default DESC, updated_at DESC`
	rows, err := r.db.Query(ctx, query, userID)
	if err != nil {
		return nil, fmt.Errorf("repository.ListAddresses: %w", err)
	}
	defer rows.Close()

	out := []models.UserAddress{}
	for rows.Next() {
		a, err := r.scanAddress(rows)
		if err != nil {
			return nil, fmt.Errorf("repository.ListAddresses.Scan: %w", err)
		}
		out = append(out, *a)
	}
	return out, nil
}

func (r *Repository) GetByID(ctx context.Context, userID string, id int64) (*models.UserAddress, error) {
	query := `SELECT ` + addressColumns + ` FROM user_addresses WHERE id = $1 AND user_id = $2`
	row := r.db.QueryRow(ctx, query, id, userID)
	a, err := r.scanAddress(row)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, models.ErrNotFound
		}
		return nil, fmt.Errorf("repository.GetAddress: %w", err)
	}
	return a, nil
}

func (r *Repository) Create(ctx context.Context, userID string, req models.CreateAddressRequest) (*models.UserAddress, error) {
	tx, err := r.db.Begin(ctx)
	if err != nil {
		return nil, fmt.Errorf("repository.CreateAddress.BeginTx: %w", err)
	}
	defer tx.Rollback(ctx) //nolint:errcheck

	if req.IsDefault {
		if err := clearDefault(tx, ctx, userID); err != nil {
			return nil, err
		}
	}

	query := `
		INSERT INTO user_addresses (user_id, recipient, line1, line2, city, region, postal_code, country, phone, is_default)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10)
		RETURNING ` + addressColumns
	args := []any{
		userID, req.Recipient, req.Line1, nullable(req.Line2), req.City,
		nullable(req.Region), nullable(req.PostalCode), req.Country, nullable(req.Phone), req.IsDefault,
	}
	a, err := r.scanAddress(tx.QueryRow(ctx, query, args...))
	if err != nil {
		return nil, fmt.Errorf("repository.CreateAddress: %w", err)
	}
	if err := tx.Commit(ctx); err != nil {
		return nil, fmt.Errorf("repository.CreateAddress.Commit: %w", err)
	}
	return a, nil
}

func (r *Repository) Update(ctx context.Context, userID string, id int64, req models.UpdateAddressRequest) (*models.UserAddress, error) {
	tx, err := r.db.Begin(ctx)
	if err != nil {
		return nil, fmt.Errorf("repository.UpdateAddress.BeginTx: %w", err)
	}
	defer tx.Rollback(ctx) //nolint:errcheck

	if req.IsDefault {
		if err := clearDefault(tx, ctx, userID); err != nil {
			return nil, err
		}
	}

	query := `
		UPDATE user_addresses
		SET recipient = $3, line1 = $4, line2 = $5, city = $6, region = $7,
		    postal_code = $8, country = $9, phone = $10, is_default = $11, updated_at = NOW()
		WHERE id = $1 AND user_id = $2
		RETURNING ` + addressColumns
	args := []any{
		id, userID, req.Recipient, req.Line1, nullable(req.Line2), req.City,
		nullable(req.Region), nullable(req.PostalCode), req.Country, nullable(req.Phone), req.IsDefault,
	}
	a, err := r.scanAddress(tx.QueryRow(ctx, query, args...))
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, models.ErrNotFound
		}
		return nil, fmt.Errorf("repository.UpdateAddress: %w", err)
	}
	if err := tx.Commit(ctx); err != nil {
		return nil, fmt.Errorf("repository.UpdateAddress.Commit: %w", err)
	}
	return a, nil
}

func (r *Repository) Delete(ctx context.Context, userID string, id int64) error {
	cmd, err := r.db.Exec(ctx, `DELETE FROM user_addresses WHERE id = $1 AND user_id = $2`, id, userID)
	if err != nil {
		return fmt.Errorf("repository.DeleteAddress: %w", err)
	}
	if cmd.RowsAffected() == 0 {
		return models.ErrNotFound
	}
	return nil
}

// SetDefault makes `id` the user's default address, unsetting any previous
// default in the same transaction. Enforced at the DB layer too via the partial
// unique index idx_user_addresses_default_per_user.
func (r *Repository) SetDefault(ctx context.Context, userID string, id int64) error {
	tx, err := r.db.Begin(ctx)
	if err != nil {
		return fmt.Errorf("repository.SetDefault.BeginTx: %w", err)
	}
	defer tx.Rollback(ctx) //nolint:errcheck

	if err := clearDefault(tx, ctx, userID); err != nil {
		return err
	}

	cmd, err := tx.Exec(ctx,
		`UPDATE user_addresses SET is_default = TRUE, updated_at = NOW() WHERE id = $1 AND user_id = $2`,
		id, userID)
	if err != nil {
		return fmt.Errorf("repository.SetDefault.Update: %w", err)
	}
	if cmd.RowsAffected() == 0 {
		return models.ErrNotFound
	}

	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("repository.SetDefault.Commit: %w", err)
	}
	return nil
}

// clearDefault unsets is_default on all of a user's addresses within tx.
func clearDefault(tx pgx.Tx, ctx context.Context, userID string) error {
	if _, err := tx.Exec(ctx,
		`UPDATE user_addresses SET is_default = FALSE WHERE user_id = $1 AND is_default = TRUE`,
		userID); err != nil {
		return fmt.Errorf("repository.clearDefault: %w", err)
	}
	return nil
}

// nullable returns nil for an empty string so pgx writes SQL NULL.
func nullable(s string) any {
	if s == "" {
		return nil
	}
	return s
}
