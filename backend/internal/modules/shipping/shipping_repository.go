package shipping

import (
	"context"
	"fmt"

	"jingdezhen-ceramics-backend/internal/models"
	"jingdezhen-ceramics-backend/internal/platform/shipping"

	"github.com/jackc/pgx/v5/pgxpool"
)

// RepositoryInterface defines shipping_fee_tiers storage.
type RepositoryInterface interface {
	ListByCountry(ctx context.Context, country string) ([]shipping.Tier, error)
	// ListAll returns all tiers (admin view). Ordered by country, then weight.
	ListAll(ctx context.Context) ([]models.ShippingFeeTier, error)
	Create(ctx context.Context, country string, maxWeightGrams int, feeCNY int64) (*models.ShippingFeeTier, error)
	Update(ctx context.Context, id int64, country string, maxWeightGrams int, feeCNY int64) (*models.ShippingFeeTier, error)
	Delete(ctx context.Context, id int64) error
}

type Repository struct {
	db *pgxpool.Pool
}

func NewRepository(db *pgxpool.Pool) RepositoryInterface {
	return &Repository{db: db}
}

func (r *Repository) ListByCountry(ctx context.Context, country string) ([]shipping.Tier, error) {
	rows, err := r.db.Query(ctx, `
		SELECT country, max_weight_grams, fee_cny
		FROM shipping_fee_tiers
		WHERE country = UPPER($1)
		ORDER BY max_weight_grams ASC`, country)
	if err != nil {
		return nil, fmt.Errorf("shipping.Repository.ListByCountry: %w", err)
	}
	defer rows.Close()
	out := []shipping.Tier{}
	for rows.Next() {
		var t shipping.Tier
		if err := rows.Scan(&t.Country, &t.MaxWeightGrams, &t.FeeCNY); err != nil {
			return nil, fmt.Errorf("shipping.Repository.ListByCountry.Scan: %w", err)
		}
		out = append(out, t)
	}
	return out, rows.Err()
}

func (r *Repository) ListAll(ctx context.Context) ([]models.ShippingFeeTier, error) {
	rows, err := r.db.Query(ctx, `
		SELECT id, country, max_weight_grams, fee_cny, created_at, updated_at
		FROM shipping_fee_tiers
		ORDER BY country ASC, max_weight_grams ASC`)
	if err != nil {
		return nil, fmt.Errorf("shipping.Repository.ListAll: %w", err)
	}
	defer rows.Close()
	out := []models.ShippingFeeTier{}
	for rows.Next() {
		var t models.ShippingFeeTier
		if err := rows.Scan(&t.ID, &t.Country, &t.MaxWeightGrams, &t.FeeCNY, &t.CreatedAt, &t.UpdatedAt); err != nil {
			return nil, fmt.Errorf("shipping.Repository.ListAll.Scan: %w", err)
		}
		out = append(out, t)
	}
	return out, rows.Err()
}

func (r *Repository) Create(ctx context.Context, country string, maxWeightGrams int, feeCNY int64) (*models.ShippingFeeTier, error) {
	var t models.ShippingFeeTier
	err := r.db.QueryRow(ctx, `
		INSERT INTO shipping_fee_tiers (country, max_weight_grams, fee_cny)
		VALUES (UPPER($1), $2, $3)
		RETURNING id, country, max_weight_grams, fee_cny, created_at, updated_at`,
		country, maxWeightGrams, feeCNY).Scan(
		&t.ID, &t.Country, &t.MaxWeightGrams, &t.FeeCNY, &t.CreatedAt, &t.UpdatedAt)
	if err != nil {
		return nil, fmt.Errorf("shipping.Repository.Create: %w", err)
	}
	return &t, nil
}

func (r *Repository) Update(ctx context.Context, id int64, country string, maxWeightGrams int, feeCNY int64) (*models.ShippingFeeTier, error) {
	var t models.ShippingFeeTier
	err := r.db.QueryRow(ctx, `
		UPDATE shipping_fee_tiers
		SET country = UPPER($2), max_weight_grams = $3, fee_cny = $4, updated_at = NOW()
		WHERE id = $1
		RETURNING id, country, max_weight_grams, fee_cny, created_at, updated_at`,
		id, country, maxWeightGrams, feeCNY).Scan(
		&t.ID, &t.Country, &t.MaxWeightGrams, &t.FeeCNY, &t.CreatedAt, &t.UpdatedAt)
	if err != nil {
		return nil, fmt.Errorf("shipping.Repository.Update: %w", err)
	}
	return &t, nil
}

func (r *Repository) Delete(ctx context.Context, id int64) error {
	cmd, err := r.db.Exec(ctx, `DELETE FROM shipping_fee_tiers WHERE id = $1`, id)
	if err != nil {
		return fmt.Errorf("shipping.Repository.Delete: %w", err)
	}
	if cmd.RowsAffected() == 0 {
		return models.ErrNotFound
	}
	return nil
}
