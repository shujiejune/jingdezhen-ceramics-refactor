package media_test

import (
	"context"
	"testing"

	"jingdezhen-ceramics-backend/internal/models"
	"jingdezhen-ceramics-backend/internal/modules/media"
	"jingdezhen-ceramics-backend/internal/testutil"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Integration tests for the entity galleries (artist / ceramic-story / activity).
// They exercise the DB-backed generic helpers (attachGallery / listGallery /
// detachGallery / reorderGallery) against a real, fully-migrated PG via
// testcontainers. The artist gallery is the representative case; the story +
// activity galleries share the exact same code path (only the table + FK column
// differ), so one entity's coverage validates the shared helpers.

// TestArtistGallery_FullLifecycle proves attach (append-last) → list (ordered)
// → reorder → list (reordered) → detach → list (empty) end-to-end.
func TestArtistGallery_FullLifecycle(t *testing.T) {
	if testing.Short() {
		t.Skip("integration: needs testcontainers PG")
	}
	pool := testutil.NewDBPool(t)
	ctx := context.Background()
	repo := media.NewRepository(pool)
	svc := media.NewService(repo, nil) // nil store → PublicURL stays empty (fine for lifecycle)

	// Seed an artist + a media asset (the artist FK on media_assets.uploaded_by
	// is nullable; we bypass the artist FK entirely by inserting a bare asset).
	artistID := seedArtist(t, pool)
	media1 := seedAsset(t, pool, "image/jpeg", "artists/a1.jpg")
	media2 := seedAsset(t, pool, "image/jpeg", "artists/a2.jpg")
	media3 := seedAsset(t, pool, "image/jpeg", "artists/a3.jpg")

	// Attach three (no sort_order → append-last: 0,1,2).
	require.NoError(t, svc.AttachToArtist(ctx, artistID, media1, nil, "first"))
	require.NoError(t, svc.AttachToArtist(ctx, artistID, media2, nil, ""))
	require.NoError(t, svc.AttachToArtist(ctx, artistID, media3, nil, "third"))

	// List → ordered 0,1,2 with captions preserved + media joined in.
	items, err := svc.ListArtistMedia(ctx, artistID)
	require.NoError(t, err)
	require.Len(t, items, 3)
	assert.Equal(t, int64(0), items[0].SortOrder)
	assert.Equal(t, media1, items[0].MediaID)
	assert.Equal(t, "first", *items[0].Caption)
	assert.Equal(t, media2, items[1].MediaID)
	assert.Equal(t, int64(2), items[2].SortOrder)
	assert.Equal(t, "artists/a3.jpg", items[2].MediaAsset.OSSKey)

	// Reorder: reverse the order.
	require.NoError(t, svc.ReorderArtistMedia(ctx, artistID, []models.ReorderMediaItem{
		{MediaID: media3, SortOrder: 0},
		{MediaID: media2, SortOrder: 1},
		{MediaID: media1, SortOrder: 2},
	}))
	items, err = svc.ListArtistMedia(ctx, artistID)
	require.NoError(t, err)
	require.Len(t, items, 3)
	assert.Equal(t, media3, items[0].MediaID)
	assert.Equal(t, media1, items[2].MediaID)

	// Detach the middle one.
	require.NoError(t, svc.DetachFromArtist(ctx, artistID, media2))
	items, err = svc.ListArtistMedia(ctx, artistID)
	require.NoError(t, err)
	require.Len(t, items, 2)

	// Detach a non-attached media → ErrNotFound.
	err = svc.DetachFromArtist(ctx, artistID, media2)
	assert.ErrorIs(t, err, models.ErrNotFound)

	// Attach again with an explicit sort_order (ON CONFLICT updates it).
	so := 5
	require.NoError(t, svc.AttachToArtist(ctx, artistID, media1, &so, "moved"))
	items, err = svc.ListArtistMedia(ctx, artistID)
	require.NoError(t, err)
	// media1 now has sort_order 5; it sorts last (5 > the others' 0/2).
	require.Len(t, items, 2)
	assert.Equal(t, int64(5), items[1].SortOrder)
	assert.Equal(t, "moved", *items[1].Caption)
}

// TestStoryGallery_ReorderAndDetach proves the same path on ceramic_story_media
// (different table + FK column) — confirms the generic helper is wired for the
// story entity, not just artist.
func TestStoryGallery_ReorderAndDetach(t *testing.T) {
	if testing.Short() {
		t.Skip("integration: needs testcontainers PG")
	}
	pool := testutil.NewDBPool(t)
	ctx := context.Background()
	repo := media.NewRepository(pool)
	svc := media.NewService(repo, nil)

	storyID := seedStory(t, pool)
	m1 := seedAsset(t, pool, "image/jpeg", "stories/s1.jpg")
	m2 := seedAsset(t, pool, "image/jpeg", "stories/s2.jpg")

	require.NoError(t, svc.AttachToStory(ctx, storyID, m1, nil, ""))
	require.NoError(t, svc.AttachToStory(ctx, storyID, m2, nil, ""))
	require.NoError(t, svc.ReorderStoryMedia(ctx, storyID, []models.ReorderMediaItem{
		{MediaID: m2, SortOrder: 0},
		{MediaID: m1, SortOrder: 1},
	}))
	items, err := svc.ListStoryMedia(ctx, storyID)
	require.NoError(t, err)
	require.Len(t, items, 2)
	assert.Equal(t, m2, items[0].MediaID)
	require.NoError(t, svc.DetachFromStory(ctx, storyID, m1))
}

// TestActivityGallery_AttachDetach proves the activity path.
func TestActivityGallery_AttachDetach(t *testing.T) {
	if testing.Short() {
		t.Skip("integration: needs testcontainers PG")
	}
	pool := testutil.NewDBPool(t)
	ctx := context.Background()
	repo := media.NewRepository(pool)
	svc := media.NewService(repo, nil)

	activityID := seedActivity(t, pool)
	m := seedAsset(t, pool, "image/jpeg", "activities/act1.jpg")
	require.NoError(t, svc.AttachToActivity(ctx, activityID, m, nil, "hero"))
	items, err := svc.ListActivityMedia(ctx, activityID)
	require.NoError(t, err)
	require.Len(t, items, 1)
	assert.Equal(t, "hero", *items[0].Caption)
	require.NoError(t, svc.DetachFromActivity(ctx, activityID, m))
	items, _ = svc.ListActivityMedia(ctx, activityID)
	assert.Empty(t, items)
}

// TestEntityGallery_CascadeDeleteParent proves deleting the parent entity
// removes its gallery rows (ON DELETE CASCADE).
func TestEntityGallery_CascadeDeleteParent(t *testing.T) {
	if testing.Short() {
		t.Skip("integration: needs testcontainers PG")
	}
	pool := testutil.NewDBPool(t)
	ctx := context.Background()
	repo := media.NewRepository(pool)
	svc := media.NewService(repo, nil)

	artistID := seedArtist(t, pool)
	m := seedAsset(t, pool, "image/jpeg", "artists/cascade.jpg")
	require.NoError(t, svc.AttachToArtist(ctx, artistID, m, nil, ""))

	// Delete the parent artist row directly → gallery rows cascade away.
	_, err := pool.Exec(ctx, `DELETE FROM artists WHERE id=$1`, artistID)
	require.NoError(t, err)
	items, _ := svc.ListArtistMedia(ctx, artistID)
	assert.Empty(t, items, "deleting the artist cascaded the gallery clean")
}

// --- seed helpers ---

func seedArtist(t *testing.T, pool *pgxpool.Pool) int64 {
	t.Helper()
	var id int64
	// artists has no NOT NULL cols except defaults; insert a bare row.
	err := pool.QueryRow(context.Background(),
		`INSERT INTO artists (display_order) VALUES (0) RETURNING id`).Scan(&id)
	require.NoError(t, err)
	return id
}

func seedStory(t *testing.T, pool *pgxpool.Pool) int64 {
	t.Helper()
	var id int64
	err := pool.QueryRow(context.Background(),
		`INSERT INTO ceramic_stories (display_order) VALUES (0) RETURNING id`).Scan(&id)
	require.NoError(t, err)
	return id
}

func seedActivity(t *testing.T, pool *pgxpool.Pool) int64 {
	t.Helper()
	var id int64
	err := pool.QueryRow(context.Background(),
		`INSERT INTO activities (type) VALUES ('Destination') RETURNING id`).Scan(&id)
	require.NoError(t, err)
	return id
}

func seedAsset(t *testing.T, pool *pgxpool.Pool, mime, ossKey string) int64 {
	t.Helper()
	var id int64
	err := pool.QueryRow(context.Background(),
		`INSERT INTO media_assets (kind, oss_key, mime) VALUES ('image', $1, $2) RETURNING id`,
		ossKey, mime).Scan(&id)
	require.NoError(t, err)
	return id
}
