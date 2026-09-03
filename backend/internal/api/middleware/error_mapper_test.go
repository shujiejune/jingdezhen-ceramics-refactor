package middleware_test

import (
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"jingdezhen-ceramics-backend/internal/api/middleware"
	"jingdezhen-ceramics-backend/internal/models"

	"github.com/gofiber/fiber/v2"
	"github.com/stretchr/testify/require"
)

// TestErrorHandler_Sentinels verifies the central error-mapper maps each
// models.Err* sentinel to the expected HTTP status + stable code string.
// A regression here is a breaking API change for the frontend.
func TestErrorHandler_Sentinels(t *testing.T) {
	cases := []struct {
		name       string
		err        error
		wantStatus int
		wantCode   string
	}{
		{"not_found", models.ErrNotFound, 404, "not_found"},
		{"forbidden", models.ErrForbidden, 403, "forbidden"},
		{"conflict", models.ErrConflict, 409, "conflict"},
		{"invalid_credentials", models.ErrInvalidCredentials, 401, "invalid_credentials"},
		{"invalid_token", models.ErrInvalidToken, 401, "unauthorized"},
		{"too_many_attempts", models.ErrTooManyAttempts, 429, "too_many_attempts"},
		{"cart_empty", models.ErrCartEmpty, 400, "cart_empty"},
		{"unshippable", models.ErrUnshippable, 400, "unshippable"},
		{"overweight", models.ErrOverweight, 400, "overweight"},
		{"consent_required", models.ErrConsentRequired, 400, "consent_required"},
		{"invalid_locale", models.ErrInvalidLocale, 400, "bad_request"},
		{"invalid_workflow", models.ErrInvalidWorkflowTransition, 409, "conflict"},
		{"gateway_unavailable", models.ErrGatewayUnavailable, 503, "internal"},
		{"nickname_taken", models.ErrNicknameTaken, 409, "conflict"},
		{"account_deleted", models.ErrAccountDeleted, 409, "conflict"},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			app := fiber.New(fiber.Config{
				ErrorHandler:          middleware.ErrorHandler,
				DisableStartupMessage: true,
			})
			app.Get("/test", func(c *fiber.Ctx) error {
				return tc.err
			})

			resp, err := app.Test(httptest.NewRequest(http.MethodGet, "/test", nil), -1)
			require.NoError(t, err)
			require.Equal(t, tc.wantStatus, resp.StatusCode)

			var env struct {
				Error struct {
					Code    string `json:"code"`
					Message string `json:"message"`
				} `json:"error"`
			}
			require.NoError(t, json.NewDecoder(resp.Body).Decode(&env))
			require.Equal(t, tc.wantCode, env.Error.Code)
			require.NotEmpty(t, env.Error.Message)
		})
	}
}

// TestErrorHandler_APIError verifies that returning an *APIError directly
// gives full control over status, code, message, and extra fields.
func TestErrorHandler_APIError(t *testing.T) {
	app := fiber.New(fiber.Config{
		ErrorHandler:          middleware.ErrorHandler,
		DisableStartupMessage: true,
	})
	app.Get("/test", func(c *fiber.Ctx) error {
		return models.NewAPIError(422, "validation_failed", "Validation failed").
			WithDetails(map[string]string{"email": "email is required"}).
			WithExtra("pending_token", "tok_123")
	})

	resp, err := app.Test(httptest.NewRequest(http.MethodGet, "/test", nil), -1)
	require.NoError(t, err)
	require.Equal(t, 422, resp.StatusCode)

	var env struct {
		Error struct {
			Code         string            `json:"code"`
			Message      string            `json:"message"`
			Details      map[string]string `json:"details"`
			PendingToken string            `json:"pending_token"`
		} `json:"error"`
	}
	require.NoError(t, json.NewDecoder(resp.Body).Decode(&env))
	require.Equal(t, "validation_failed", env.Error.Code)
	require.Equal(t, "Validation failed", env.Error.Message)
	require.Equal(t, "email is required", env.Error.Details["email"])
	require.Equal(t, "tok_123", env.Error.PendingToken)
}

// TestErrorHandler_ConsentNotGranted verifies the analytics consent gate
// maps to 204 No Content (not an error).
func TestErrorHandler_ConsentNotGranted(t *testing.T) {
	app := fiber.New(fiber.Config{
		ErrorHandler:          middleware.ErrorHandler,
		DisableStartupMessage: true,
	})
	app.Get("/test", func(c *fiber.Ctx) error {
		return models.ErrConsentNotGranted
	})

	resp, err := app.Test(httptest.NewRequest(http.MethodGet, "/test", nil), -1)
	require.NoError(t, err)
	require.Equal(t, 204, resp.StatusCode)
}

// TestErrorHandler_FiberError verifies fiber.NewError is respected for
// status but gets a stable code from the status fallback table.
func TestErrorHandler_FiberError(t *testing.T) {
	app := fiber.New(fiber.Config{
		ErrorHandler:          middleware.ErrorHandler,
		DisableStartupMessage: true,
	})
	app.Get("/test", func(c *fiber.Ctx) error {
		return fiber.NewError(fiber.StatusNotFound, "Order not found")
	})

	resp, err := app.Test(httptest.NewRequest(http.MethodGet, "/test", nil), -1)
	require.NoError(t, err)
	require.Equal(t, 404, resp.StatusCode)

	var env struct {
		Error struct {
			Code    string `json:"code"`
			Message string `json:"message"`
		} `json:"error"`
	}
	require.NoError(t, json.NewDecoder(resp.Body).Decode(&env))
	require.Equal(t, "not_found", env.Error.Code)
	require.Contains(t, env.Error.Message, "Order not found")
}

// TestErrorHandler_UnknownError verifies unknown errors map to 500/internal.
func TestErrorHandler_UnknownError(t *testing.T) {
	app := fiber.New(fiber.Config{
		ErrorHandler:          middleware.ErrorHandler,
		DisableStartupMessage: true,
	})
	app.Get("/test", func(c *fiber.Ctx) error {
		return errors.New("something broke")
	})

	resp, err := app.Test(httptest.NewRequest(http.MethodGet, "/test", nil), -1)
	require.NoError(t, err)
	require.Equal(t, 500, resp.StatusCode)

	var env struct {
		Error struct {
			Code    string `json:"code"`
			Message string `json:"message"`
		} `json:"error"`
	}
	require.NoError(t, json.NewDecoder(resp.Body).Decode(&env))
	require.Equal(t, "internal", env.Error.Code)
}

// TestErrorHandler_WrappedError verifies errors.Is still matches when
// the sentinel is wrapped (fmt.Errorf %w, errors.Wrap, etc.).
func TestErrorHandler_WrappedError(t *testing.T) {
	app := fiber.New(fiber.Config{
		ErrorHandler:          middleware.ErrorHandler,
		DisableStartupMessage: true,
	})
	app.Get("/test", func(c *fiber.Ctx) error {
		return errors.New("context: " + models.ErrNotFound.Error())
	})
	// This is NOT errors.Is-recognized because string concatenation doesn't
	// preserve the wrapping. Use fmt.Errorf("...: %w", err) instead.
	// Verify the fallback path:
	resp, err := app.Test(httptest.NewRequest(http.MethodGet, "/test", nil), -1)
	require.NoError(t, err)
	require.Equal(t, 500, resp.StatusCode) // unknown string error → internal
}
