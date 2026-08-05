package user

import (
	"errors"
	"fmt"
	"log"
	"time"

	"jingdezhen-ceramics-backend/internal/models"
	"jingdezhen-ceramics-backend/internal/modules/audit"
	"jingdezhen-ceramics-backend/pkg/utils"

	"github.com/go-playground/validator/v10"
	"github.com/gofiber/fiber/v2"
)

type Handler struct {
	service  ServiceInterface
	validate *validator.Validate // For request body validation
	audit    *audit.Helper
}

// NewHandler creates a new user handler.
// The AdminHandler can be this same handler, with routes protected by AdminRequired middleware.
func NewHandler(service ServiceInterface) *Handler {
	return &Handler{
		service:  service,
		validate: validator.New(),
	}
}

// SetAuditLogger injects the audit logger (PRD §3.1.1). Nil = no-op (tests).
func (h *Handler) SetAuditLogger(l audit.Logger) { h.audit = audit.NewHelper(l) }

// Signup: POST /auth/signup (public). Creates an inactive user + sends activation email.
//
// @Summary      Sign up a new user
// @Description  Creates an inactive user account + sends an activation email.
// @Description  Returns a generic 201 (the user activates via email link).
// @Tags         auth
// @Accept       json
// @Produce      json
// @Param        body body models.SignupRequest true "Signup credentials (nickname, email, password)"
// @Success      201 {object} models.AuthResponse
// @Failure      400 {object} models.ErrorResponse "Invalid body / validation"
// @Failure      409 {object} models.ErrorResponse "Email address is already in use"
// @Failure      500 {object} models.ErrorResponse "Internal error"
// @Router       /auth/signup [post]
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

// Login: POST /auth/login (public). Returns access token + profile, or a 2FA challenge.
//
// @Summary      Log in
// @Description  Authenticates by email + password. If 2FA is enabled, returns a
// @Description  pending_token (HTTP 200 with a pending_token field) instead of a real
// @Description  access token; the client completes via /auth/2fa/verify.
// @Description  Super_admins without 2FA enrolled are blocked (must enroll via pending flow).
// @Tags         auth
// @Accept       json
// @Produce      json
// @Param        body body models.LoginRequest true "Login credentials (email, password)"
// @Success      200 {object} models.AuthResponse "access_token (real or pending_token for 2FA)"
// @Failure      401 {object} models.ErrorResponse "Invalid credentials / account not active"
// @Failure      500 {object} models.ErrorResponse "Internal error"
// @Router       /auth/login [post]
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
					"code":          "2fa_required",
					"message":       "Two-factor authentication code required",
					"pending_token": authResponse.AccessToken,
				},
			})
		}
		if errors.Is(err, models.Err2FAEnrollmentRequired) {
			// Super admin with no 2FA enrolled: blocked from a full session. The
			// frontend walks the user through /auth/2fa/pending-enroll →
			// /auth/2fa/pending-confirm (which mints the real JWT on success).
			return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{
				"error": fiber.Map{
					"code":          "2fa_enrollment_required",
					"message":       "Two-factor enrollment required before you can log in",
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
//
// @Summary      Complete 2FA login
// @Description  Completes a 2FA-challenged login. Public (no JWT): the pending token +
// @Description  6-digit TOTP code are the credentials. On success mints the real access token.
// @Tags         auth,2fa
// @Accept       json
// @Produce      json
// @Param        body body models.VerifyTwoFARequest true "pending_token + 6-digit code"
// @Success      200 {object} models.AuthResponse
// @Failure      400 {object} models.ErrorResponse "Invalid body / validation"
// @Failure      401 {object} models.ErrorResponse "Invalid or expired pending token / invalid TOTP code"
// @Failure      500 {object} models.ErrorResponse "Internal error"
// @Router       /auth/2fa/verify [post]
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
		if errors.Is(err, models.ErrTooManyAttempts) {
			return c.Status(fiber.StatusTooManyRequests).JSON(models.ErrorResponse{Message: "Too many failed attempts, try again later"})
		}
		if errors.Is(err, models.ErrInvalidCredentials) {
			return c.Status(fiber.StatusUnauthorized).JSON(models.ErrorResponse{Message: "Invalid TOTP code"})
		}
		log.Printf("Handler.Verify2FALogin: %v", err)
		return c.Status(fiber.StatusInternalServerError).JSON(models.ErrorResponse{Message: "Failed to complete 2FA login"})
	}
	return c.Status(fiber.StatusOK).JSON(authResponse)
}

// Pending2FAEnroll: POST /auth/2fa/pending-enroll — must-enroll flow for a
// super_admin whose login was blocked (Err2FAEnrollmentRequired). PUBLIC: the
// pending token is the credential. Returns the otpauth:// URI + raw secret.
//
// @Summary      Start 2FA enrollment (must-enroll flow)
// @Description  Begins 2FA enrollment for a super_admin blocked at login. Public
// @Description  (the pending token is the credential). Returns the otpauth:// URI + raw secret.
// @Tags         auth,2fa
// @Accept       json
// @Produce      json
// @Param        body body models.PendingTwoFAEnrollRequest true "pending_token + password"
// @Success      200 {object} object "otpauth URI + raw secret"
// @Failure      400 {object} models.ErrorResponse "Invalid body / validation"
// @Failure      401 {object} models.ErrorResponse "Invalid or expired pending token"
// @Failure      500 {object} models.ErrorResponse "Internal error"
// @Router       /auth/2fa/pending-enroll [post]
func (h *Handler) Pending2FAEnroll(c *fiber.Ctx) error {
	var req models.PendingTwoFAEnrollRequest
	if err := c.BodyParser(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(models.ErrorResponse{Message: "Invalid request body"})
	}
	if err := h.validate.Struct(req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(models.ErrorResponse{Message: "Validation failed: " + err.Error()})
	}
	resp, err := h.service.StartPending2FAEnrollment(c.Context(), req.PendingToken, req)
	if err != nil {
		if errors.Is(err, models.ErrInvalidToken) {
			return c.Status(fiber.StatusUnauthorized).JSON(models.ErrorResponse{Message: "Invalid or expired pending token"})
		}
		log.Printf("Handler.Pending2FAEnroll: %v", err)
		return c.Status(fiber.StatusInternalServerError).JSON(models.ErrorResponse{Message: "Failed to start 2FA enrollment"})
	}
	return c.Status(fiber.StatusOK).JSON(resp)
}

// Pending2FAConfirm: POST /auth/2fa/pending-confirm — must-enroll flow. PUBLIC.
// Verifies the first TOTP code, enables 2FA, and completes login (mints the
// real access token). On success the super_admin is logged in with 2FA on.
//
// @Summary      Confirm 2FA enrollment + complete login
// @Description  Verifies the first TOTP code, enables 2FA, mints the real access token,
// @Description  and returns backup codes. Public (pending token is the credential).
// @Tags         auth,2fa
// @Accept       json
// @Produce      json
// @Param        body body models.PendingTwoFAConfirmRequest true "pending_token + 6-digit code"
// @Success      200 {object} object "AuthResponse + backup_codes"
// @Failure      400 {object} models.ErrorResponse "Invalid body / validation"
// @Failure      401 {object} models.ErrorResponse "Invalid or expired pending token / invalid TOTP code"
// @Failure      500 {object} models.ErrorResponse "Internal error"
// @Router       /auth/2fa/pending-confirm [post]
func (h *Handler) Pending2FAConfirm(c *fiber.Ctx) error {
	var req models.PendingTwoFAConfirmRequest
	if err := c.BodyParser(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(models.ErrorResponse{Message: "Invalid request body"})
	}
	if err := h.validate.Struct(req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(models.ErrorResponse{Message: "Validation failed: " + err.Error()})
	}
	authResponse, backupCodes, err := h.service.Complete2FAEnrollment(c.Context(), req.PendingToken, req.Code)
	if err != nil {
		if errors.Is(err, models.ErrInvalidToken) {
			return c.Status(fiber.StatusUnauthorized).JSON(models.ErrorResponse{Message: "Invalid or expired pending token"})
		}
		if errors.Is(err, models.ErrInvalidCredentials) {
			return c.Status(fiber.StatusUnauthorized).JSON(models.ErrorResponse{Message: "Invalid TOTP code"})
		}
		if errors.Is(err, models.ErrNotFound) {
			return c.Status(fiber.StatusBadRequest).JSON(models.ErrorResponse{Message: "No pending enrollment — call /auth/2fa/pending-enroll first"})
		}
		log.Printf("Handler.Pending2FAConfirm: %v", err)
		return c.Status(fiber.StatusInternalServerError).JSON(models.ErrorResponse{Message: "Failed to complete 2FA enrollment"})
	}
	return c.Status(fiber.StatusOK).JSON(models.TwoFAEnrollmentCompleteResponse{
		AuthResponse: authResponse,
		BackupCodes:  backupCodes,
	})
}

// ActivateAccount: POST /auth/activate (public). Activates an inactive user via
// email token + auto-logs them in.
//
// @Summary      Activate a user account
// @Description  Activates an inactive account via the email activation token, then
// @Description  auto-logs the user in (mints a JWT).
// @Tags         auth
// @Accept       json
// @Produce      json
// @Param        body body models.ActivationRequest true "Activation token (from email)"
// @Success      200 {object} models.AuthResponse
// @Failure      400 {object} models.ErrorResponse "Invalid or expired activation token"
// @Failure      500 {object} models.ErrorResponse "Internal error"
// @Router       /auth/activate [post]
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
//
// @Summary      Resend the activation email
// @Description  Resends the activation email. Always returns a generic success message
// @Description  (to prevent email enumeration) regardless of whether the email exists.
// @Tags         auth
// @Accept       json
// @Produce      json
// @Param        body body models.ResendActivationRequest true "Email to resend activation to"
// @Success      200 {object} object "{message: \"...\"}"
// @Failure      400 {object} models.ErrorResponse "Invalid body / validation"
// @Router       /auth/resend-activation [post]
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
//
// @Summary      Start Google OAuth login
// @Description  Redirects the browser to Google's consent screen (sets an oauthstate cookie).
// @Tags         auth,oauth
// @Produce      json
// @Success      307 {string} string "Redirect to Google consent screen"
// @Failure      500 {object} models.ErrorResponse "Could not initiate Google login"
// @Router       /auth/google/login [get]
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
//
// @Summary      Google OAuth callback
// @Description  Handles the Google OAuth callback: exchanges the code for a token,
// @Description  finds/creates the user, and either mints a JWT or issues a 2FA challenge.
// @Description  Redirects to the frontend with the token (or a 2FA pending_token).
// @Tags         auth,oauth
// @Produce      json
// @Param        state query string true "OAuth state (must match the oauthstate cookie)"
// @Param        code  query string true "Authorization code from Google"
// @Success      307 {string} string "Redirect to frontend /login/success (or /2fa, /2fa/enroll, /error)"
// @Failure      400 {object} models.ErrorResponse "Authorization code not provided"
// @Failure      401 {object} models.ErrorResponse "Invalid or missing state cookie / state mismatch"
// @Router       /auth/google/callback [get]
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
	// find or create the user, and either generate the JWT or issue a 2FA
	// pending-token challenge (TDD §5.3 — Google login is 2FA-gated too).
	authResponse, err := h.service.HandleGoogleCallback(c.Context(), code)
	if err != nil {
		if errors.Is(err, models.Err2FARequired) {
			// 2FA already enabled — frontend collects the TOTP code and POSTs to
			// /auth/2fa/verify to complete login.
			return c.Redirect(fmt.Sprintf("%s/login/2fa?pending_token=%s", h.service.GetClientOrigin(), authResponse.AccessToken), fiber.StatusTemporaryRedirect)
		}
		if errors.Is(err, models.Err2FAEnrollmentRequired) {
			// Super admin must enroll — frontend walks through pending-enroll →
			// pending-confirm (which mints the real JWT on success).
			return c.Redirect(fmt.Sprintf("%s/login/2fa/enroll?pending_token=%s", h.service.GetClientOrigin(), authResponse.AccessToken), fiber.StatusTemporaryRedirect)
		}
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
//
// @Summary      Request a password reset (step 1)
// @Description  Sends a password-reset email if the account exists. Always returns a generic
// @Description  success message (anti-enumeration) regardless of whether the email exists.
// @Tags         auth
// @Accept       json
// @Produce      json
// @Param        body body models.RequestPasswordResetRequest true "Email to reset"
// @Success      200 {object} object "{message: \"...\"}"
// @Failure      400 {object} models.ErrorResponse "Invalid body / validation"
// @Router       /auth/request-password-reset [post]
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
//
// @Summary      Reset password (step 2)
// @Description  Completes a password reset: verifies the token, sets the new password,
// @Description  and mints a new JWT (auto-login).
// @Tags         auth
// @Accept       json
// @Produce      json
// @Param        body body models.ResetPasswordRequest true "Reset token + new password (min 8)"
// @Success      200 {object} models.AuthResponse
// @Failure      400 {object} models.ErrorResponse "Invalid body / validation / invalid or expired token"
// @Failure      500 {object} models.ErrorResponse "Internal error"
// @Router       /auth/reset-password [post]
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
//
// @Summary      Get the current user's profile
// @Description  Returns the signed-in user's profile.
// @Tags         profile
// @Accept       json
// @Produce      json
// @Param        Authorization header string true "Bearer <access_token>"
// @Success      200 {object} models.User
// @Failure      401 {object} models.ErrorResponse "Authentication required"
// @Failure      404 {object} models.ErrorResponse "User profile not found"
// @Failure      500 {object} models.ErrorResponse "Internal error"
// @Security     BearerAuth
// @Router       /profile [get]
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
	//
	// @Summary      Update the current user's profile
	// @Description  Updates editable profile fields (nickname, avatar, contacts, preferred
	// @Description  locale/currency). Nil pointers = unchanged.
	// @Tags         profile
	// @Accept       json
	// @Produce      json
	// @Param        Authorization header string true "Bearer <access_token>"
	// @Param        body body models.UserUpdateData true "Fields to update (nil pointers = unchanged)"
	// @Success      200 {object} models.User
	// @Failure      400 {object} models.ErrorResponse "Invalid body / validation"
	// @Failure      401 {object} models.ErrorResponse "Authentication required"
	// @Failure      404 {object} models.ErrorResponse "User profile not found"
	// @Failure      500 {object} models.ErrorResponse "Internal error"
	// @Security     BearerAuth
	// @Router       /profile [put]
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
	//
	// @Summary      Submit the contact form
	// @Description  Public endpoint: sends a contact/feedback submission via email.
	// @Tags         contact
	// @Accept       json
	// @Produce      json
	// @Param        body body models.ContactFormData true "Contact form payload"
	// @Success      200 {object} object "{message: \"Contact form submitted successfully\"}"
	// @Failure      400 {object} models.ErrorResponse "Invalid body / validation"
	// @Failure      500 {object} models.ErrorResponse "Internal error"
	// @Router       /contact [post]
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
//
// @Summary      List users (admin)
// @Description  Paginated list of all users. Access: super_admin (users.manage).
// @Tags         admin,users
// @Accept       json
// @Produce      json
// @Param        Authorization header string true "Bearer <access_token>"
// @Param        page  query int false "Page number (1-based)" default(1)
// @Param        limit query int false "Page size (max 100)" default(20)
// @Success      200 {object} models.PaginatedResponse{data=[]models.User}
// @Failure      401 {object} models.ErrorResponse "Authentication required"
// @Failure      403 {object} models.ErrorResponse "Forbidden (needs users.manage)"
// @Failure      500 {object} models.ErrorResponse "Internal error"
// @Security     BearerAuth
// @Router       /admin/users [get]
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
	//
	// @Summary      Assign a role to a user (admin)
	// @Description  Sets a user's role. Access: super_admin (users.manage).
	// @Tags         admin,users
	// @Accept       json
	// @Produce      json
	// @Param        Authorization header string true "Bearer <access_token>"
	// @Param        user_id path string true "Target user ID (UUID)"
	// @Param        body body object true "{role: <one of super_admin|content_editor|travel_planner|ecommerce_operator|customer_service>}"
	// @Success      200 {object} object "{message: \"User role updated successfully\"}"
	// @Failure      400 {object} models.ErrorResponse "Invalid body / validation"
	// @Failure      401 {object} models.ErrorResponse "Authentication required"
	// @Failure      403 {object} models.ErrorResponse "Forbidden (needs users.manage)"
	// @Failure      404 {object} models.ErrorResponse "Target user not found"
	// @Failure      500 {object} models.ErrorResponse "Internal error"
	// @Security     BearerAuth
	// @Router       /admin/users/{user_id}/role [put]
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
	h.audit.Log(c, models.AuditActionRoleAssign, models.AuditEntityUser, targetUserID, map[string]any{"role": req.Role})
	return c.Status(fiber.StatusOK).JSON(map[string]string{"message": "User role updated successfully"})
}
