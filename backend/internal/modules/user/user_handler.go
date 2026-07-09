package user

import (
	"errors"
	"fmt"
	"jingdezhen-ceramics-backend/internal/models"
	"jingdezhen-ceramics-backend/pkg/utils"
	"log"
	"time"

	"github.com/go-playground/validator/v10"
	"github.com/gofiber/fiber/v2"
)

type Handler struct {
	service  ServiceInterface
	validate *validator.Validate // For request body validation
}

// NewHandler creates a new user handler.
// The AdminHandler can be this same handler, with routes protected by AdminRequired middleware.
func NewHandler(service ServiceInterface) *Handler {
	return &Handler{
		service:  service,
		validate: validator.New(),
	}
}

func (h *Handler) Signup(c *fiber.Ctx) error {
	var req models.SignupRequest
	if err := c.BodyParser(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(models.ErrorResponse{Message: "Invalid request body"})
	}
	if err := h.validate.Struct(req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(models.ErrorResponse{Message: "Validation failed: " + err.Error()})
	}

	authResponse, err := h.service.Signup(c.Context(), req)
	if err != nil {
		if errors.Is(err, models.ErrConflict) {
			return c.Status(fiber.StatusConflict).JSON(models.ErrorResponse{Message: "Email address is already in use"})
		}
		log.Printf("Handler.Signup: %v", err)
		return c.Status(fiber.StatusInternalServerError).JSON(models.ErrorResponse{Message: "Failed to create user"})
	}

	return c.Status(fiber.StatusCreated).JSON(authResponse)
}

func (h *Handler) Login(c *fiber.Ctx) error {
	var req models.LoginRequest
	if err := c.BodyParser(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(models.ErrorResponse{Message: "Invalid request body"})
	}
	if err := h.validate.Struct(req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(models.ErrorResponse{Message: "Validation failed: " + err.Error()})
	}

	authResponse, err := h.service.Login(c.Context(), req)
	if err != nil {
		if errors.Is(err, models.ErrInvalidCredentials) { // Define this error in models
			return c.Status(fiber.StatusUnauthorized).JSON(models.ErrorResponse{Message: "Invalid email or password"})
		}
		if errors.Is(err, models.Err2FARequired) {
			// Password OK; a TOTP code is required to complete login. The pending
			// token is in authResponse.AccessToken; the frontend POSTs it + the
			// 6-digit code to /auth/2fa/verify.
			return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{
				"error": fiber.Map{
					"code":         "2fa_required",
					"message":      "Two-factor authentication code required",
					"pending_token": authResponse.AccessToken,
				},
			})
		}
		log.Printf("Handler.Login: %v", err)
		return c.Status(fiber.StatusInternalServerError).JSON(models.ErrorResponse{Message: "Failed to log in"})
	}

	return c.Status(fiber.StatusOK).JSON(authResponse)
}

// Verify2FALogin: POST /auth/2fa/verify — complete a login challenged for TOTP.
// PUBLIC (no JWT); the pending token + 6-digit code are the credentials. On
// success the user service mints the real access token + full profile (TDD §5.3).
func (h *Handler) Verify2FALogin(c *fiber.Ctx) error {
	var req models.VerifyTwoFARequest
	if err := c.BodyParser(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(models.ErrorResponse{Message: "Invalid request body"})
	}
	if err := h.validate.Struct(req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(models.ErrorResponse{Message: "Validation failed: " + err.Error()})
	}
	authResponse, err := h.service.Complete2FALogin(c.Context(), req.PendingToken, req.Code)
	if err != nil {
		if errors.Is(err, models.ErrInvalidToken) {
			return c.Status(fiber.StatusUnauthorized).JSON(models.ErrorResponse{Message: "Invalid or expired pending token"})
		}
		if errors.Is(err, models.ErrInvalidCredentials) {
			return c.Status(fiber.StatusUnauthorized).JSON(models.ErrorResponse{Message: "Invalid TOTP code"})
		}
		log.Printf("Handler.Verify2FALogin: %v", err)
		return c.Status(fiber.StatusInternalServerError).JSON(models.ErrorResponse{Message: "Failed to complete 2FA login"})
	}
	return c.Status(fiber.StatusOK).JSON(authResponse)
}

func (h *Handler) ActivateAccount(c *fiber.Ctx) error {
	var req models.ActivationRequest
	if err := c.BodyParser(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(models.ErrorResponse{Message: "Invalid request: missing token"})
	}
	if err := h.validate.Struct(req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(models.ErrorResponse{Message: err.Error()})
	}

	// After activation, automatically log the user in by issuing a JWT
	fmt.Println("Activation Token: ", req.Token)
	authResponse, err := h.service.ActivateUserAndLogin(c.Context(), req.Token)
	if err != nil {
		if errors.Is(err, models.ErrInvalidToken) {
			return c.Status(fiber.StatusBadRequest).JSON(models.ErrorResponse{Message: "Invalid or expired activation token"})
		}
		log.Printf("Handler.ActivateAccount: %v", err)
		return c.Status(fiber.StatusInternalServerError).JSON(models.ErrorResponse{Message: "Failed to activate account"})
	}

	return c.Status(fiber.StatusOK).JSON(authResponse)
}

// ResendActivation handles requests to resend an activation email.
func (h *Handler) ResendActivation(c *fiber.Ctx) error {
	var req models.ResendActivationRequest
	if err := c.BodyParser(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(models.ErrorResponse{Message: "Invalid request: " + err.Error()})
	}
	if err := h.validate.Struct(req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(models.ErrorResponse{Message: "Validation failed: " + err.Error()})
	}

	err := h.service.ResendActivationEmail(c.Context(), req.Email)
	if err != nil {
		// Even if the service returns an error, don't expose it to the client
		// to prevent email enumeration. The error is logged in the service layer.
		log.Printf("Handler.ResendActivation encountered a service error: %v", err)
	}

	// Always return a generic success message to prevent attackers from discovering which emails are registered.
	return c.Status(fiber.StatusOK).JSON(map[string]string{
		"message": "If an account with that email address exists and is not yet activated, a new activation link has been sent.",
	})
}

// GoogleLogin initiates the Google OAuth 2.0 login flow.
// It redirects the user to Google's consent screen.
func (h *Handler) GoogleLogin(c *fiber.Ctx) error {
	// The service generates the unique URL for this login attempt.
	// This URL includes the client ID and a state parameter for security.
	authURL, state, err := h.service.HandleGoogleLogin()
	if err != nil {
		log.Printf("Handler.GoogleLogin: failed to generate auth URL: %v", err)
		return c.Status(fiber.StatusInternalServerError).JSON(models.ErrorResponse{Message: "Could not initiate Google login"})
	}

	// Create a new secure cookie to store the state parameter
	c.Cookie(&fiber.Cookie{
		Name:     "oauthstate", // Name of the cookie
		Value:    state,
		Expires:  time.Now().Add(10 * time.Minute), // Cookie is valid for 10 minutes
		HTTPOnly: true,                             // Prevents JavaScript from accessing the cookie
		Secure:   true,                             // Only send over HTTPS (set to false in config for localhost HTTP dev)
		SameSite: "Lax",
	})

	// Redirect the user's browser to the Google authentication page.
	return c.Redirect(authURL, fiber.StatusTemporaryRedirect)
}

// GoogleCallback handles the callback request from Google after the user has authenticated,
// and validates the state parameter from the URL against the one stored in the cookie.
// Google redirects the user here with a `code` and `state` parameter in the URL.
func (h *Handler) GoogleCallback(c *fiber.Ctx) error {
	// 1. Read the state from the cookie set in the login step.
	oauthStateCookie := c.Cookies("oauthstate")
	if oauthStateCookie == "" {
		// If the cookie expired or was never set
		log.Printf("Handler.GoogleCallback: could not read state cookie")
		return c.Status(fiber.StatusUnauthorized).JSON(models.ErrorResponse{Message: "Invalid or missing state cookie"})
	}

	// 2. Compare the state from the cookie with the state from the query parameter.
	if c.Query("state") != oauthStateCookie {
		log.Printf("Handler.GoogleCallback: state parameter mismatch")
		return c.Status(fiber.StatusUnauthorized).JSON(models.ErrorResponse{Message: "Invalid state parameter"})
	}

	// 3. Delete the cookie after it has been used once.
	c.Cookie(&fiber.Cookie{
		Name:    "oauthstate",
		Expires: time.Now().Add(-time.Hour), // Set expiration to the past
	})

	// 4. Get the authorization code from the query parameters.
	code := c.Query("code")
	if code == "" {
		return c.Status(fiber.StatusBadRequest).JSON(models.ErrorResponse{Message: "Authorization code not provided"})
	}

	// 5. Call the service to exchange the code for a token, fetch user info,
	// find or create the user, and generate the application's JWT.
	authResponse, err := h.service.HandleGoogleCallback(c.Context(), code)
	if err != nil {
		log.Printf("Handler.GoogleCallback: service error: %v", err)
		// Redirect to a frontend error page
		return c.Redirect(fmt.Sprintf("%s/login/error", h.service.GetClientOrigin()), fiber.StatusTemporaryRedirect)
	}

	// 6. Redirect the user back to a specific frontend page with the token.
	// The frontend page can then parse the token from the URL and save it.
	redirectURL := fmt.Sprintf("%s/login/success?token=%s", h.service.GetClientOrigin(), authResponse.AccessToken)
	return c.Redirect(redirectURL, fiber.StatusTemporaryRedirect)
}

// RequestPasswordReset handles requests to initiate a password reset.
// Two-step password reset process:
// 1. User clicks "Forgot password", frontend sends a POST request to "auth/reset-password"
// 2. User submits new password on frontend page "/reset-password?token=...", frontend sends a POST request with new password
// This is the step 1
func (h *Handler) RequestPasswordReset(c *fiber.Ctx) error {
	var req models.RequestPasswordResetRequest
	if err := c.BodyParser(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(models.ErrorResponse{Message: "Invalid request: " + err.Error()})
	}
	if err := h.validate.Struct(req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(models.ErrorResponse{Message: "Validation failed: " + err.Error()})
	}

	err := h.service.RequestPasswordReset(c.Context(), req.Email)
	if err != nil {
		// As with activation, we log the error but don't expose it to the client.
		log.Printf("Handler.RequestPasswordReset encountered a service error: %v", err)
	}

	// Always return a generic success message.
	return c.Status(fiber.StatusOK).JSON(map[string]string{
		"message": "If an account with that email address exists, a link to reset your password has been sent.",
	})
}

// This is the step 2
// It receives a token and a new password, validates them, and if successful,
// logs the user in by returning a new JWT.
func (h *Handler) ResetPassword(c *fiber.Ctx) error {
	// 1. Bind the incoming JSON request body to our ResetPasswordRequest struct.
	var req models.ResetPasswordRequest
	if err := c.BodyParser(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(models.ErrorResponse{Message: "Invalid request body: " + err.Error()})
	}

	// 2. Validate the request data using the struct tags
	if err := h.validate.Struct(req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(models.ErrorResponse{Message: "Validation failed: " + err.Error()})
	}

	// 3. Call the corresponding service method to perform the core logic.
	// The service will verify the token, hash the new password, update the database,
	// and generate a new JWT.
	fmt.Println("Reset Password Token: ", req.Token)
	authResponse, err := h.service.ResetPassword(c.Context(), req.Token, req.NewPassword)
	if err != nil {
		// 4. Handle specific errors returned from the service layer.
		if errors.Is(err, models.ErrInvalidToken) {
			// This error is returned if the token doesn't exist, is expired, or is otherwise invalid.
			return c.Status(fiber.StatusBadRequest).JSON(models.ErrorResponse{Message: "Invalid or expired password reset token"})
		}

		// For all other unexpected errors, log them and return a generic server error.
		log.Printf("Handler.ResetPassword: %v", err)
		return c.Status(fiber.StatusInternalServerError).JSON(models.ErrorResponse{Message: "An internal error occurred while resetting the password"})
	}

	// 5. On success, the service returns a new AuthResponse.
	return c.Status(fiber.StatusOK).JSON(authResponse)
}

// --- User Profile Routes ---
func (h *Handler) GetProfile(c *fiber.Ctx) error {
	userID, err := utils.GetUserIDFromContext(c)
	if err != nil {
		return c.Status(fiber.StatusUnauthorized).JSON(models.ErrorResponse{Message: err.Error()})
	}

	user, err := h.service.GetUserProfile(c.Context(), userID)
	if err != nil {
		if errors.Is(err, models.ErrNotFound) {
			return c.Status(fiber.StatusNotFound).JSON(models.ErrorResponse{Message: "User profile not found"})
		}
		log.Printf("Handler.GetProfile: %v", err)
		return c.Status(fiber.StatusInternalServerError).JSON(models.ErrorResponse{Message: "Failed to retrieve profile"})
	}
	return c.Status(fiber.StatusOK).JSON(user)
}

func (h *Handler) UpdateProfile(c *fiber.Ctx) error {
	userID, err := utils.GetUserIDFromContext(c)
	if err != nil {
		return c.Status(fiber.StatusUnauthorized).JSON(models.ErrorResponse{Message: err.Error()})
	}

	var req models.UserUpdateData
	if err := c.BodyParser(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(models.ErrorResponse{Message: "Invalid request body: " + err.Error()})
	}
	if err := h.validate.Struct(req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(models.ErrorResponse{Message: "Validation failed: " + err.Error()})
	}

	user, err := h.service.UpdateUserProfile(c.Context(), userID, req)
	if err != nil {
		if errors.Is(err, models.ErrNotFound) {
			return c.Status(fiber.StatusNotFound).JSON(models.ErrorResponse{Message: "User profile not found"})
		}
		log.Printf("Handler.UpdateProfile: %v", err)
		return c.Status(fiber.StatusInternalServerError).JSON(models.ErrorResponse{Message: "Failed to update profile"})
	}
	return c.Status(fiber.StatusOK).JSON(user)
}

func (h *Handler) SubmitContactForm(c *fiber.Ctx) error {
	var req models.ContactFormData
	if err := c.BodyParser(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(models.ErrorResponse{Message: "Invalid request body: " + err.Error()})
	}
	if err := h.validate.Struct(req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(models.ErrorResponse{Message: "Validation failed: " + err.Error()})
	}

	err := h.service.HandleContactSubmission(c.Context(), req)
	if err != nil {
		log.Printf("Handler.SubmitContactForm: %v", err)
		return c.Status(fiber.StatusInternalServerError).JSON(models.ErrorResponse{Message: "Failed to submit contact form"})
	}
	return c.Status(fiber.StatusOK).JSON(map[string]string{"message": "Contact form submitted successfully"})
}

// --- Admin User Management Routes ---
// Protected in router.go by middleware.RequirePermission(models.PermUsersManage).
func (h *Handler) AdminListUsers(c *fiber.Ctx) error {
	page, limit := utils.GetPageLimit(c)
	users, total, err := h.service.AdminListUsers(c.Context(), page, limit)
	if err != nil {
		log.Printf("Handler.AdminListUsers: %v", err)
		return c.Status(fiber.StatusInternalServerError).JSON(models.ErrorResponse{Message: "Failed to list users"})
	}
	return c.Status(fiber.StatusOK).JSON(models.NewPaginatedResponse(users, page, limit, total))
}

func (h *Handler) AdminAssignRole(c *fiber.Ctx) error {
	targetUserID := c.Params("user_id")
	var req struct {
		Role string `json:"role" validate:"required,oneof=super_admin content_editor travel_planner ecommerce_operator customer_service"`
	}
	if err := c.BodyParser(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(models.ErrorResponse{Message: "Invalid request body: " + err.Error()})
	}
	if err := h.validate.Struct(req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(models.ErrorResponse{Message: "Validation failed: " + err.Error()})
	}

	err := h.service.AdminAssignRole(c.Context(), targetUserID, req.Role)
	if err != nil {
		if errors.Is(err, models.ErrNotFound) {
			return c.Status(fiber.StatusNotFound).JSON(models.ErrorResponse{Message: "Target user not found"})
		}
		log.Printf("Handler.AdminAssignRole: %v", err)
		return c.Status(fiber.StatusInternalServerError).JSON(models.ErrorResponse{Message: "Failed to assign role"})
	}
	return c.Status(fiber.StatusOK).JSON(map[string]string{"message": "User role updated successfully"})
}
