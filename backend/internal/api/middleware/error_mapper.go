package middleware

import (
	"encoding/json"
	"errors"
	"log"
	"net/http"

	"jingdezhen-ceramics-backend/internal/models"

	"github.com/gofiber/fiber/v2"
)

// ErrorEnvelope is the wire shape for all error responses (TDD §4.3, §5.1):
//
//	{"error":{"code":"not_found","message":"...","details":{...}}}
//
// Extra fields (e.g. pending_token for 2FA) are merged into the inner map.
type ErrorEnvelope struct {
	Error map[string]any `json:"error"`
}

// ErrorHandler is the central Fiber error handler (set via fiber.Config{}).
// It replaces the per-handler `if errors.Is(err, models.ErrX) { ... }` chains
// with a single mapping table in models.MapErrorToAPIError.
//
// Handlers can:
//  1. `return err` — the mapper infers status + code from the sentinel.
//  2. `return models.NewAPIError(404, "not_found", "Order not found")` —
//     full control, entity-specific message.
//  3. `return fiber.NewError(fiber.StatusNotFound, "custom msg")` — Fiber's
//     built-in typed error; the mapper respects the status but infers the
//     code from the status fallback table.
//
// Special case: models.ErrConsentNotGranted maps to 204 No Content (the
// analytics consent gate silently drops the event, not a client error).
func ErrorHandler(ctx *fiber.Ctx, err error) error {
	// Special case: analytics consent gate returns 204 (not an error).
	if errors.Is(err, models.ErrConsentNotGranted) {
		return ctx.SendStatus(http.StatusNoContent)
	}

	// Special case: fiber.Error (handlers using fiber.NewError for explicit
	// statuses). Respect the status; infer a code from the status fallback.
	var fiberErr *fiber.Error
	if errors.As(err, &fiberErr) {
		code := codeFromStatus(fiberErr.Code)
		return writeError(ctx, fiberErr.Code, code, fiberErr.Message, nil, nil)
	}

	// General case: map via the sentinel table or unwrap an *APIError.
	apiErr := models.MapErrorToAPIError(err)
	if apiErr == nil {
		return ctx.SendStatus(http.StatusInternalServerError)
	}

	return writeError(ctx, apiErr.Status, apiErr.Code, apiErr.Message, apiErr.Details, apiErr.Extra)
}

// writeError emits the error envelope as JSON and sets the HTTP status.
func writeError(ctx *fiber.Ctx, status int, code, message string, details map[string]string, extra map[string]any) error {
	inner := map[string]any{
		"code":    code,
		"message": message,
	}
	if len(details) > 0 {
		inner["details"] = details
	}
	for k, v := range extra {
		inner[k] = v
	}

	ctx.Status(status)
	ctx.Set("Content-Type", "application/json")
	body, err := json.Marshal(ErrorEnvelope{Error: inner})
	if err != nil {
		log.Printf("ErrorHandler: marshal error: %v", err)
		return ctx.SendStatus(http.StatusInternalServerError)
	}
	return ctx.Send(body)
}

// codeFromStatus maps an HTTP status code to a stable error code string,
// matching the frontend's statusCodeFallback (frontend/src/lib/api.ts).
func codeFromStatus(status int) string {
	switch {
	case status >= 500:
		return "internal"
	case status == http.StatusUnauthorized:
		return "unauthorized"
	case status == http.StatusForbidden:
		return "forbidden"
	case status == http.StatusNotFound:
		return "not_found"
	case status == http.StatusConflict:
		return "conflict"
	case status == http.StatusTooManyRequests:
		return "too_many_attempts"
	case status == http.StatusUnprocessableEntity:
		return "validation_failed"
	default:
		return "bad_request"
	}
}
