package models

import (
	"encoding/json"
	"errors"
	"net/http"
)

// APIError is a structured error that carries a stable machine-readable code,
// an HTTP status, a human-readable message, and optional extra fields (e.g.
// pending_token for 2FA challenges). Handlers and services can return an
// *APIError to bypass the error-mapper's sentinel-to-code inference and
// control the full envelope.
//
// The error-mapper (middleware/error_mapper.go) is the single place that
// converts APIError + sentinel errors into the wire envelope:
//
//	{"error":{"code":"...","message":"...","details":"..."}}
//
// See TDD §4.3 + §5.1.
type APIError struct {
	Status  int               `json:"-"`                 // HTTP status code (not serialized)
	Code    string            `json:"code"`              // stable snake_case code
	Message string            `json:"message"`           // human-readable
	Details map[string]string `json:"details,omitempty"` // optional field-key map (e.g. validation errors)
	Extra   map[string]any    `json:"-"`                 // extra top-level fields merged into error (e.g. pending_token)
}

func (e *APIError) Error() string {
	if e.Message != "" {
		return e.Message
	}
	return e.Code
}

// NewAPIError builds an APIError with the given status, code, and message.
func NewAPIError(status int, code, message string) *APIError {
	return &APIError{Status: status, Code: code, Message: message}
}

// WithExtra attaches a key→value pair to the error's Extra map (for
// fields like pending_token that ride on the error envelope).
func (e *APIError) WithExtra(key string, value any) *APIError {
	if e.Extra == nil {
		e.Extra = make(map[string]any)
	}
	e.Extra[key] = value
	return e
}

// WithDetails attaches a field→message map (for validation_failed with
// per-field error messages).
func (e *APIError) WithDetails(fields map[string]string) *APIError {
	e.Details = fields
	return e
}

// MarshalJSON serializes the error into the wire envelope shape:
// {"error":{"code":...,"message":...,"details":...,"pending_token":...}}
// Extra fields are merged at the same level as code/message.
func (e *APIError) MarshalJSON() ([]byte, error) {
	inner := map[string]any{
		"code":    e.Code,
		"message": e.Message,
	}
	if len(e.Details) > 0 {
		inner["details"] = e.Details
	}
	for k, v := range e.Extra {
		inner[k] = v
	}
	return json.Marshal(map[string]any{"error": inner})
}

// ErrToAPIError maps a sentinel error to an APIError with a stable code
// and HTTP status. This is the central mapping table (TDD §4.3). Returns
// nil if the error is not a recognized sentinel (the caller should fall
// back to 500/internal).
//
// The stable codes match the frontend's expected strings (see
// frontend/src/lib/api.ts classifyApiError + i18n/en-US.ts errors.*).
var sentinelMap = []struct {
	err     error
	status  int
	code    string
	message string
}{
	{ErrInvalidCredentials, http.StatusUnauthorized, "invalid_credentials", "Invalid email or password"},
	{Err2FARequired, http.StatusUnauthorized, "2fa_required", "Two-factor authentication code required"},
	{Err2FAEnrollmentRequired, http.StatusUnauthorized, "2fa_enrollment_required", "Two-factor enrollment required before you can log in"},
	{ErrInvalidToken, http.StatusUnauthorized, "unauthorized", "Invalid or expired JWT"},
	{ErrTooManyAttempts, http.StatusTooManyRequests, "too_many_attempts", "Too many failed attempts, try again later"},
	{ErrForbidden, http.StatusForbidden, "forbidden", "You do not have permission to access this resource"},
	{ErrNotOwned, http.StatusForbidden, "forbidden", "You do not have permission to access this resource"},
	{ErrNotFound, http.StatusNotFound, "not_found", "Resource not found"},
	{ErrInactiveAccount, http.StatusForbidden, "forbidden", "Account is not active"},
	{ErrAccountDeleted, http.StatusConflict, "conflict", "Account has been deleted"},
	{ErrConflict, http.StatusConflict, "conflict", "Resource conflict"},
	{ErrNicknameTaken, http.StatusConflict, "conflict", "Nickname already taken"},
	{ErrInvalidOperation, http.StatusBadRequest, "bad_request", "Operation is not valid for this resource"},
	{ErrInvalidLocale, http.StatusBadRequest, "bad_request", "Unsupported locale"},
	{ErrInvalidWorkflowTransition, http.StatusConflict, "conflict", "Invalid content workflow transition for this actor"},
	{ErrCartEmpty, http.StatusBadRequest, "cart_empty", "Cart is empty; cannot check out"},
	{ErrUnshippable, http.StatusBadRequest, "unshippable", "Destination country is not shippable"},
	{ErrOverweight, http.StatusBadRequest, "overweight", "Order exceeds the maximum shipping weight for the destination"},
	{ErrConsentRequired, http.StatusBadRequest, "consent_required", "Privacy policy consent is required"},
	{ErrWebhookSignatureInvalid, http.StatusBadRequest, "bad_request", "Webhook signature verification failed"},
	{ErrGatewayUnavailable, http.StatusServiceUnavailable, "internal", "Payment gateway is not configured"},
	{ErrRateNotFound, http.StatusBadRequest, "bad_request", "FX rate not found for currency"},
	{ErrRequestNotQuoted, http.StatusConflict, "conflict", "Itinerary request is not in a quoted state"},
	{ErrQuoteNotPayable, http.StatusConflict, "conflict", "Quote is not in a payable state"},
	{ErrQuoteAlreadyPaid, http.StatusConflict, "conflict", "Deposit has already been paid for this quote"},
	{ErrInvalidQuote, http.StatusBadRequest, "bad_request", "Quote line items are invalid"},
	{ErrItineraryNotCancellable, http.StatusConflict, "conflict", "Itinerary request is not in a cancellable state"},
	{ErrPaymentNotSucceeded, http.StatusConflict, "conflict", "No succeeded payment found for the order"},
	{ErrMissedDeadline, http.StatusConflict, "conflict", "Deadline missed"},
	{ErrLimitExceeded, http.StatusTooManyRequests, "too_many_attempts", "Submission or usage limit exceeded"},
	{ErrInvalidForumPostCategoryID, http.StatusBadRequest, "bad_request", "Invalid category"},
}

// MapErrorToAPIError converts a service-layer error into an *APIError.
// If the error is already an *APIError, it's returned as-is. If it's a
// recognized sentinel, the sentinelMap is consulted. Otherwise the error
// is treated as an internal server error (500/internal).
func MapErrorToAPIError(err error) *APIError {
	if err == nil {
		return nil
	}

	// Already an APIError — unwrap and return.
	var apiErr *APIError
	if errors.As(err, &apiErr) {
		return apiErr
	}

	// Sentinel mapping.
	for _, m := range sentinelMap {
		if errors.Is(err, m.err) {
			return &APIError{
				Status:  m.status,
				Code:    m.code,
				Message: m.message,
			}
		}
	}

	// Unknown error → 500.
	return &APIError{
		Status:  http.StatusInternalServerError,
		Code:    "internal",
		Message: "An unexpected error occurred",
	}
}
