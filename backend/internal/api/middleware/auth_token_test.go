package middleware_test

import (
	"context"
	"net/http/httptest"
	"testing"
	"time"

	"jingdezhen-ceramics-backend/internal/api/middleware"
	"jingdezhen-ceramics-backend/internal/testutil"
	"jingdezhen-ceramics-backend/pkg/adapters/tokenblocklist"

	"github.com/gofiber/fiber/v2"
	"github.com/golang-jwt/jwt/v5"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const testSecret = "test-jwt-secret"

// mintToken builds a signed HS256 JWT for the given user_id (mirrors
// user_service.generateAuthResponse). Roles default to empty (customer).
func mintToken(t *testing.T, userID string) string {
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

// authedApp builds a Fiber app whose single route is gated by JWTMAuth(bl).
func authedApp(bl tokenblocklist.Blocklist) *fiber.App {
	app := fiber.New()
	g := app.Group("/protected")
	g.Use(middleware.JWTMAuth(testSecret, bl))
	g.Get("/me", func(c *fiber.Ctx) error {
		return c.SendStatus(fiber.StatusOK)
	})
	return app
}

func TestJWTMAuth_NilBlocklistAllowsValidToken(t *testing.T) {
	app := authedApp(nil) // nil blocklist = Noop skip
	tok := mintToken(t, "u-valid")
	req := httptest.NewRequest("GET", "/protected/me", nil)
	req.Header.Set("Authorization", "Bearer "+tok)
	resp, err := app.Test(req)
	require.NoError(t, err)
	assert.Equal(t, 200, resp.StatusCode)
}

func TestJWTMAuth_RevokedTokenRejected(t *testing.T) {
	if testing.Short() {
		t.Skip("integration test skipped under -short")
	}
	client := testutil.NewRedisClient(t)
	bl := tokenblocklist.NewRedisBlocklist(client)
	ctx := context.Background()

	// Revoke u-deleted BEFORE the request.
	require.NoError(t, bl.Revoke(ctx, "u-deleted", tokenblocklist.MaxAccessTokenTTL))

	app := authedApp(bl)

	// Valid token for a REVOKED user → 401.
	tok := mintToken(t, "u-deleted")
	req := httptest.NewRequest("GET", "/protected/me", nil)
	req.Header.Set("Authorization", "Bearer "+tok)
	resp, err := app.Test(req)
	require.NoError(t, err)
	assert.Equal(t, 401, resp.StatusCode)

	// Valid token for a NON-revoked user → 200.
	tok2 := mintToken(t, "u-active")
	req2 := httptest.NewRequest("GET", "/protected/me", nil)
	req2.Header.Set("Authorization", "Bearer "+tok2)
	resp2, err := app.Test(req2)
	require.NoError(t, err)
	assert.Equal(t, 200, resp2.StatusCode)
}

func TestJWTMAuth_FailOpenOnRedisOutage(t *testing.T) {
	if testing.Short() {
		t.Skip("integration test skipped under -short")
	}
	// A closed client simulates a Redis outage: IsRevoked fail-opens to false,
	// so a valid token for a (would-be-revoked) user still passes.
	client := testutil.NewRedisClient(t)
	require.NoError(t, client.Close())
	bl := tokenblocklist.NewRedisBlocklist(client)

	app := authedApp(bl)
	tok := mintToken(t, "u-anything")
	req := httptest.NewRequest("GET", "/protected/me", nil)
	req.Header.Set("Authorization", "Bearer "+tok)
	resp, err := app.Test(req)
	require.NoError(t, err)
	assert.Equal(t, 200, resp.StatusCode, "fail-open: valid token passes during Redis outage")
}

func TestJWTMAuth_MissingTokenRejected(t *testing.T) {
	app := authedApp(nil)
	req := httptest.NewRequest("GET", "/protected/me", nil)
	resp, err := app.Test(req)
	require.NoError(t, err)
	assert.Equal(t, 401, resp.StatusCode)
}
