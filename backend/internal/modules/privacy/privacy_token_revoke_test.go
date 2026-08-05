package privacy_test

import (
	"context"
	"net/http/httptest"
	"testing"
	"time"

	"jingdezhen-ceramics-backend/internal/api/middleware"
	"jingdezhen-ceramics-backend/internal/models"
	"jingdezhen-ceramics-backend/internal/modules/privacy"
	"jingdezhen-ceramics-backend/internal/platform/jobs"
	"jingdezhen-ceramics-backend/internal/testutil"
	"jingdezhen-ceramics-backend/pkg/adapters/tokenblocklist"

	"github.com/gofiber/fiber/v2"
	"github.com/golang-jwt/jwt/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const testSecret = "test-jwt-secret-privacy"

// noopEmail is a no-op EmailEnqueuer so the privacy service can run in tests
// without an Asynq instance.
type noopEmail struct{}

func (noopEmail) EnqueueEmailSend(_ context.Context, _ jobs.EmailSendPayload) error {
	return nil
}

func mintPrivacyToken(t *testing.T, userID string) string {
	t.Helper()
	claims := jwt.MapClaims{
		"user_id": userID,
		"email":   "x@jingdezhen.test",
		"roles":   []string{},
		"exp":     jwt.NewNumericDate(time.Now().Add(time.Hour)),
	}
	tok := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	s, err := tok.SignedString([]byte(testSecret))
	require.NoError(t, err)
	return s
}

// seedPrivacyUser inserts an active customer + returns its UUID.
func seedPrivacyUser(t *testing.T) (*pgxpool.Pool, string) {
	db := testutil.NewDBPool(t)
	ctx := context.Background()
	var userID string
	err := db.QueryRow(ctx, `
		INSERT INTO users (email, nickname, is_active, auth_provider, password_hash)
		VALUES ('privacy-revoke@jingdezhen.test', 'Privacy Test', true, 'email', 'x')
		RETURNING id::text`).Scan(&userID)
	require.NoError(t, err)
	return db, userID
}

// TestDeleteAccount_RevokesOutstandingToken is the priority end-to-end check
// (TDD §11 spirit): a deleted user's still-valid JWT must be rejected by the
// auth middleware, not linger up to the 30d expiry.
func TestDeleteAccount_RevokesOutstandingToken(t *testing.T) {
	if testing.Short() {
		t.Skip("integration test skipped under -short")
	}
	db, userID := seedPrivacyUser(t)
	redisClient := testutil.NewRedisClient(t)
	ctx := context.Background()

	bl := tokenblocklist.NewRedisBlocklist(redisClient)
	repo := privacy.NewRepository(db)
	svc := privacy.NewService(repo, noopEmail{}, bl)

	// Mint a token for the live user — it's valid (signature + expiry).
	tok := mintPrivacyToken(t, userID)

	// Build a minimal authed endpoint guarded by the same JWTMAuth + blocklist.
	app := fiber.New()
	g := app.Group("/protected")
	g.Use(middleware.JWTMAuth(testSecret, bl))
	g.Get("/me", func(c *fiber.Ctx) error { return c.SendStatus(fiber.StatusOK) })

	// Before erasure: the token works.
	req := httptest.NewRequest("GET", "/protected/me", nil)
	req.Header.Set("Authorization", "Bearer "+tok)
	resp, err := app.Test(req)
	require.NoError(t, err)
	assert.Equal(t, 200, resp.StatusCode, "valid token works before erasure")

	// Erase the account (privacy service).
	require.NoError(t, svc.DeleteAccount(ctx, userID))

	// The blocklist now reports the user as revoked.
	revoked, err := bl.IsRevoked(ctx, userID)
	require.NoError(t, err)
	assert.True(t, revoked, "erasure wrote a revocation entry")

	// After erasure: the SAME valid token is rejected (401).
	req2 := httptest.NewRequest("GET", "/protected/me", nil)
	req2.Header.Set("Authorization", "Bearer "+tok)
	resp2, err := app.Test(req2)
	require.NoError(t, err)
	assert.Equal(t, 401, resp2.StatusCode, "deleted user's outstanding token is invalidated")

	// The user row was anonymized (is_active=false, deleted_at set).
	var isActive bool
	var deletedAt *time.Time
	err = db.QueryRow(ctx, `SELECT is_active, deleted_at FROM users WHERE id = $1`, userID).
		Scan(&isActive, &deletedAt)
	require.NoError(t, err)
	assert.False(t, isActive)
	require.NotNil(t, deletedAt)
}

// TestDeleteAccount_NilRevokerSkipsRevocation confirms the best-effort seam:
// a nil TokenRevoker (worker / no-Redis path) does not break erasure, and the
// token simply is not denylisted (login is still blocked by is_active=false).
func TestDeleteAccount_NilRevokerSkipsRevocation(t *testing.T) {
	if testing.Short() {
		t.Skip("integration test skipped under -short")
	}
	db, userID := seedPrivacyUser(t)
	ctx := context.Background()

	repo := privacy.NewRepository(db)
	svc := privacy.NewService(repo, noopEmail{}, nil) // nil revoker

	require.NoError(t, svc.DeleteAccount(ctx, userID))

	// Erasure still completed.
	var isActive bool
	err := db.QueryRow(ctx, `SELECT is_active FROM users WHERE id = $1`, userID).Scan(&isActive)
	require.NoError(t, err)
	assert.False(t, isActive, "erasure ran even with nil revoker")
}

// _ keeps the models import alive (used for future assertions on error types).
var _ = models.ErrAccountDeleted
