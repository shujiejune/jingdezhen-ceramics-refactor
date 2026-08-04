package itinerary_test

// Integration tests for the option-rate CMS CRUD (PRD §3.3.2: operator-
// configured per-option rate table). The bug-prone bits unique to this sub-track:
//   - option_key is the canonical immutable identifier (historical quote
//     snapshots freeze it) → UPDATE must not touch option_key.
//   - option_key regex (^[a-z0-9][a-z0-9_-]*$) is enforced in the service for
//     a clear 400 before hitting SQL; the DB CHECK is the backstop.
//   - Duplicate option_key on create → 409 (UNIQUE violation surfaced by the
//     handler's constraint-name probe).
//   - NotFound on update/delete of a missing id.

import (
	"context"
	"testing"

	"jingdezhen-ceramics-backend/internal/models"
	"jingdezhen-ceramics-backend/internal/modules/itinerary"
	"jingdezhen-ceramics-backend/internal/testutil"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/require"
)

// TestCreateOptionRate_OK verifies a well-formed rate inserts + the row
// round-trips (option_key, rate_cny, unit, label all preserved).
func TestCreateOptionRate_OK(t *testing.T) {
	pool := testutil.NewDBPool(t)
	ctx := context.Background()
	repo := itinerary.NewRepository(pool)
	svc := itinerary.NewService(repo, nil, nil, nil, nil, "mock") // fx nil OK for CRUD

	o, err := svc.CreateOptionRate(ctx, models.CreateOptionRateRequest{
		OptionKey: "test-guide-premium", RateCNY: 50000,
		Unit: "per_person", DisplayLabel: "Premium guide",
	})
	require.NoError(t, err)
	require.NotZero(t, o.ID)
	require.Equal(t, "test-guide-premium", o.OptionKey)
	require.Equal(t, int64(50000), o.RateCNY)
	require.Equal(t, "per_person", o.Unit)
	require.Equal(t, "Premium guide", o.DisplayLabel)

	// The list reflects the new row.
	all, err := svc.ListOptionRates(ctx)
	require.NoError(t, err)
	require.True(t, len(all) >= 1)
}

// TestCreateOptionRate_BadKey verifies the option_key regex guard rejects
// uppercase / spaces / leading-dash keys with ErrInvalidOperation (a clear 400,
// not a SQL CHECK violation → 500).
func TestCreateOptionRate_BadKey(t *testing.T) {
	pool := testutil.NewDBPool(t)
	ctx := context.Background()
	svc := itinerary.NewService(itinerary.NewRepository(pool), nil, nil, nil, nil, "mock")

	bad := []string{"Guide-English", "guide english", "-guide", "guide!", "GUIDE"}
	for _, k := range bad {
		_, err := svc.CreateOptionRate(ctx, models.CreateOptionRateRequest{
			OptionKey: k, RateCNY: 1000, Unit: "flat",
		})
		require.ErrorIs(t, err, models.ErrInvalidOperation, "key %q should fail the regex", k)
	}
}

// TestCreateOptionRate_DuplicateKey verifies a duplicate option_key errors
// (the DB UNIQUE constraint). The handler surfaces this as 409; the service
// returns a wrapped pgx error the handler probes.
func TestCreateOptionRate_DuplicateKey(t *testing.T) {
	pool := testutil.NewDBPool(t)
	ctx := context.Background()
	repo := itinerary.NewRepository(pool)
	svc := itinerary.NewService(repo, nil, nil, nil, nil, "mock")

	key := "test-dup-key"
	_, err := svc.CreateOptionRate(ctx, models.CreateOptionRateRequest{
		OptionKey: key, RateCNY: 1000, Unit: "flat",
	})
	require.NoError(t, err)

	_, err = svc.CreateOptionRate(ctx, models.CreateOptionRateRequest{
		OptionKey: key, RateCNY: 2000, Unit: "flat",
	})
	require.Error(t, err, "duplicate option_key must error")
}

// TestUpdateOptionRate_KeyImmutable verifies the UPDATE path mutates only
// rate_cny/unit/display_label — option_key stays the same after an update.
func TestUpdateOptionRate_KeyImmutable(t *testing.T) {
	pool := testutil.NewDBPool(t)
	ctx := context.Background()
	repo := itinerary.NewRepository(pool)
	svc := itinerary.NewService(repo, nil, nil, nil, nil, "mock")

	orig, err := svc.CreateOptionRate(ctx, models.CreateOptionRateRequest{
		OptionKey: "test-immutable-key", RateCNY: 1000, Unit: "flat", DisplayLabel: "Old",
	})
	require.NoError(t, err)

	updated, err := svc.UpdateOptionRate(ctx, orig.ID, models.UpdateOptionRateRequest{
		RateCNY: 9999, Unit: "per_day", DisplayLabel: "New label",
	})
	require.NoError(t, err)
	// option_key unchanged; the mutable fields took the new values.
	require.Equal(t, "test-immutable-key", updated.OptionKey)
	require.Equal(t, int64(9999), updated.RateCNY)
	require.Equal(t, "per_day", updated.Unit)
	require.Equal(t, "New label", updated.DisplayLabel)

	// Re-read via the list to confirm the persisted row still carries the key.
	all, err := repo.ListOptionRates(ctx)
	require.NoError(t, err)
	for _, o := range all {
		if o.ID == orig.ID {
			require.Equal(t, "test-immutable-key", o.OptionKey)
		}
	}
}

// TestUpdateOptionRate_NotFound verifies a missing id → ErrNotFound.
func TestUpdateOptionRate_NotFound(t *testing.T) {
	pool := testutil.NewDBPool(t)
	svc := itinerary.NewService(itinerary.NewRepository(pool), nil, nil, nil, nil, "mock")
	_, err := svc.UpdateOptionRate(context.Background(), 999999, models.UpdateOptionRateRequest{
		RateCNY: 1000, Unit: "flat",
	})
	require.ErrorIs(t, err, models.ErrNotFound)
}

// TestDeleteOptionRate_OK verifies delete removes the row (a subsequent list
// no longer contains the id).
func TestDeleteOptionRate_OK(t *testing.T) {
	pool := testutil.NewDBPool(t)
	ctx := context.Background()
	repo := itinerary.NewRepository(pool)
	svc := itinerary.NewService(repo, nil, nil, nil, nil, "mock")

	o, err := svc.CreateOptionRate(ctx, models.CreateOptionRateRequest{
		OptionKey: "test-delete-key", RateCNY: 1000, Unit: "flat",
	})
	require.NoError(t, err)
	require.NoError(t, svc.DeleteOptionRate(ctx, o.ID))

	all, err := repo.ListOptionRates(ctx)
	require.NoError(t, err)
	for _, x := range all {
		require.NotEqual(t, o.ID, x.ID, "deleted row must not appear in the list")
	}
}

// TestDeleteOptionRate_NotFound verifies a missing id → ErrNotFound.
func TestDeleteOptionRate_NotFound(t *testing.T) {
	pool := testutil.NewDBPool(t)
	svc := itinerary.NewService(itinerary.NewRepository(pool), nil, nil, nil, nil, "mock")
	err := svc.DeleteOptionRate(context.Background(), 999999)
	require.ErrorIs(t, err, models.ErrNotFound)
}

// _ keeps the pgxpool import referenced if the helpers above are trimmed later.
var _ = (*pgxpool.Pool)(nil)
