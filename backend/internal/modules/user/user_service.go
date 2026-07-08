package user

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"jingdezhen-ceramics-backend/internal/models"
	emailSvc "jingdezhen-ceramics-backend/pkg/email"
	"jingdezhen-ceramics-backend/internal/platform/jobs"
	"jingdezhen-ceramics-backend/pkg/utils"
	"log"
	"net/http"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"golang.org/x/crypto/bcrypt"
	"golang.org/x/oauth2"
)

// ServiceInterface defines methods for user business logic.
type ServiceInterface interface {
	GetClientOrigin() string

	Signup(ctx context.Context, req models.SignupRequest) (*models.User, error)
	ActivateUserAndLogin(ctx context.Context, token string) (*models.AuthResponse, error)
	ResendActivationEmail(ctx context.Context, email string) error
	Login(ctx context.Context, req models.LoginRequest) (*models.AuthResponse, error)
	HandleGoogleLogin() (string, string, error)
	HandleGoogleCallback(ctx context.Context, code string) (*models.AuthResponse, error)
	RequestPasswordReset(ctx context.Context, email string) error
	ResetPassword(ctx context.Context, token string, newPassword string) (*models.AuthResponse, error)

	GetUserProfile(ctx context.Context, userID string) (*models.User, error)
	UpdateUserProfile(ctx context.Context, userID string, data models.UserUpdateData) (*models.User, error)
	HandleContactSubmission(ctx context.Context, data models.ContactFormData) error

	// Admin
	AdminListUsers(ctx context.Context, page, limit int) ([]models.User, int, error)
	AdminAssignRole(ctx context.Context, targetUserID string, roleKey string) error
}

// EmailEnqueuer is the subset of the Asynq job client the user service needs.
// Defined as an interface so the service is testable without a live Redis;
// *jobs.Client satisfies it. Email rendering happens at enqueue time (here),
// so the worker only needs the rendered HTML/text + Brevo sender.
type EmailEnqueuer interface {
	EnqueueEmailSend(ctx context.Context, p jobs.EmailSendPayload) error
}

type Service struct {
	userRepo          RepositoryInterface
	emailEnqueuer     EmailEnqueuer       // enqueue email:send jobs (TDD §4.2)
	templateManager   *emailSvc.TemplateManager
	jwtSecret         string
	clientOrigin      string // For sending activation and password reset emails (domain name)
	adminEmail        string
	googleOAuthConfig *oauth2.Config
}

func NewService(
	userRepo RepositoryInterface,
	emailEnqueuer EmailEnqueuer,
	tm *emailSvc.TemplateManager,
	JWTSecretFromConfig string,
	clientOriginFromConfig string,
	adminEmailFromConfig string,
	googleOAuthConfig *oauth2.Config,
) ServiceInterface {
	return &Service{
		userRepo:          userRepo,
		emailEnqueuer:     emailEnqueuer,
		templateManager:   tm,
		jwtSecret:         JWTSecretFromConfig,
		clientOrigin:      clientOriginFromConfig,
		adminEmail:        adminEmailFromConfig,
		googleOAuthConfig: googleOAuthConfig,
	}
}

// A struct to unmarshal the Google user info response
type GoogleUserInfo struct {
	ID            string `json:"id"`
	Email         string `json:"email"`
	VerifiedEmail bool   `json:"verified_email"`
	Name          string `json:"name"`
	Picture       string `json:"picture"`
}

// Allows other packages (like the handler) to know the frontend URL for redirects.
func (s *Service) GetClientOrigin() string {
	return s.clientOrigin
}

func (s *Service) Signup(ctx context.Context, req models.SignupRequest) (*models.User, error) {
	// 1. Check if user with that email already exists
	_, err := s.userRepo.FindByEmail(ctx, req.Email)
	if err != nil && !errors.Is(err, models.ErrNotFound) {
		// Some other database error occurred
		return nil, fmt.Errorf("service.Signup.FindByEmail: %w", err)
	}
	if err == nil {
		// User was found, email is taken
		return nil, models.ErrConflict
	}

	// 2. Hash the password
	hashedPassword, err := bcrypt.GenerateFromPassword([]byte(req.Password), bcrypt.DefaultCost)
	if err != nil {
		return nil, fmt.Errorf("service.Signup.HashPassword: %w", err)
	}

	// 3. Create activation token
	activationToken, err := utils.GenerateSecureToken(32)
	if err != nil {
		return nil, fmt.Errorf("service.Signup.GenerateToken: %w", err)
	}
	expiresAt := time.Now().Add(time.Minute * 30)
	log.Println("GENERATED ACTIVATION TOKEN:", activationToken)

	// 4. Create the inactive user in the database
	newUser := &models.User{
		Nickname: req.Nickname,
		Email:    req.Email,
		// New users are customers by default (no user_roles row; PRD §3.4.1).
	}
	createdUser, err := s.userRepo.CreateInactiveUser(ctx, newUser, string(hashedPassword), activationToken, expiresAt)
	if err != nil {
		return nil, fmt.Errorf("service.Signup.CreateUser: %w", err)
	}

	// 5. Send activation email
	activationURL := fmt.Sprintf("%s/activate?token=%s", s.clientOrigin, activationToken)

	htmlContent, err := s.templateManager.GenerateActivateAccountEmailHTML(emailSvc.TemplateData{
		Name: createdUser.Nickname,
		Link: activationURL,
	})
	if err != nil {
		// Log the error but don't fail the whole signup process
		log.Printf("Failed to generate activation email HTML: %v", err)
		return createdUser, nil
	}

	emailSubject := "[Jingdezhen Ceramics] Welcome! Please Activate Your Account"
	plainTextContent := fmt.Sprintf("Thank you for signing up! Please click the following link in 30 minutes to activate your account: %s", activationURL)

	// Enqueue the email send so a flaky Brevo API never blocks signup (TDD §4.2).
	if err := s.emailEnqueuer.EnqueueEmailSend(ctx, jobs.EmailSendPayload{
		To:        createdUser.Email,
		Subject:   emailSubject,
		PlainText: plainTextContent,
		HTML:      htmlContent,
	}); err != nil {
		log.Printf("Failed to enqueue activation email to %s: %v", createdUser.Email, err)
	}

	return createdUser, nil
}

// private helper function to generate AuthResponse
func (s *Service) generateAuthResponse(ctx context.Context, user *models.User) (*models.AuthResponse, error) {
	// Load the user's staff roles for the JWT claim (empty for customers).
	roles, err := s.userRepo.GetUserRoles(ctx, user.ID)
	if err != nil {
		return nil, fmt.Errorf("failed to load user roles: %w", err)
	}
	user.Roles = roles

	// 1. Create claims for JWT
	claims := &models.JwtCustomClaims{
		UserID: user.ID,
		Email:  user.Email,
		Roles:  roles,
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(time.Hour * 24 * 30)), // 1 month expiry
		},
	}

	// 2. Create access token with claims
	accessToken := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)

	// 3. Generate encoded token and send it as response
	tokenSignedString, err := accessToken.SignedString([]byte(s.jwtSecret))
	if err != nil {
		return nil, fmt.Errorf("failed to sign access token: %w", err)
	}

	user.PasswordHash = nil

	return &models.AuthResponse{
		AccessToken: tokenSignedString,
		User:        user,
	}, nil
}

func (s *Service) ActivateUserAndLogin(ctx context.Context, token string) (*models.AuthResponse, error) {
	activatedUser, err := s.userRepo.ActivateUser(ctx, token)
	if err != nil {
		return nil, fmt.Errorf("service.ActivateUserAndLogin: %w", err)
	}

	return s.generateAuthResponse(ctx, activatedUser)
}

func (s *Service) ResendActivationEmail(ctx context.Context, email string) error {
	// 1. Find user by email
	user, err := s.userRepo.FindByEmail(ctx, email)
	if err != nil {
		// If user not found, do nothing and return nil to hide existence.
		if errors.Is(err, models.ErrNotFound) {
			log.Printf("INFO: Activation resend requested for non-existent email: %s", email)
			return nil
		}
		return fmt.Errorf("service.ResendActivationEmail.FindByEmail: %w", err)
	}

	// 2. Check if user is already active
	if user.IsActive {
		log.Printf("INFO: Activation resend requested for already active user: %s", email)
		return nil // Do nothing, don't signal that they are active.
	}

	// 3. Generate a new activation token
	activationToken, err := utils.GenerateSecureToken(32)
	if err != nil {
		return fmt.Errorf("service.ResendActivationEmail.GenerateToken: %w", err)
	}
	expiresAt := time.Now().Add(time.Minute * 30)

	// 4. Update the user record with the new token
	if err := s.userRepo.UpdateActivationToken(ctx, user.ID, activationToken, expiresAt); err != nil {
		return fmt.Errorf("service.ResendActivationEmail.UpdateToken: %w", err)
	}

	// 5. Send the new activation email
	activationURL := fmt.Sprintf("%s/activate?token=%s", s.clientOrigin, activationToken)

	htmlContent, err := s.templateManager.GenerateActivateAccountEmailHTML(emailSvc.TemplateData{
		Name: user.Nickname,
		Link: activationURL,
	})
	if err != nil {
		// Log the error but don't fail the whole signup process
		log.Printf("Failed to generate re-activation email HTML: %v", err)
		return nil
	}

	emailSubject := "[Jingdezhen Ceramics] Activate Your Account (New Link)"
	plainTextContent := fmt.Sprintf("Please click the following link in 30 minutes to activate your account: %s", activationURL)

	if err := s.emailEnqueuer.EnqueueEmailSend(ctx, jobs.EmailSendPayload{
		To:        email,
		Subject:   emailSubject,
		PlainText: plainTextContent,
		HTML:      htmlContent,
	}); err != nil {
		log.Printf("Failed to enqueue re-activation email to %s: %v", email, err)
	}

	return nil
}

func (s *Service) Login(ctx context.Context, req models.LoginRequest) (*models.AuthResponse, error) {
	// 1. Find user by email
	userWithHash, err := s.userRepo.FindByEmail(ctx, req.Email) // This needs to return password hash
	if err != nil {
		if errors.Is(err, models.ErrNotFound) {
			return nil, models.ErrInvalidCredentials
		}
		return nil, fmt.Errorf("service.Login.FindByEmail: %w", err)
	}

	// 2. Compare the provided password with the stored hash
	if userWithHash.PasswordHash == nil {
		// This user was created via OAuth and has no password.
		return nil, models.ErrInvalidCredentials
	}

	err = bcrypt.CompareHashAndPassword([]byte(*userWithHash.PasswordHash), []byte(req.Password))
	if err != nil {
		// Passwords don't match
		return nil, models.ErrInvalidCredentials
	}

	// 3. Check if the user is active
	if !userWithHash.IsActive {
		return nil, models.ErrInactiveAccount
	}

	// 4. Use helper function to generate JWT and AuthResponse
	return s.generateAuthResponse(ctx, userWithHash)
}

func (s *Service) RequestPasswordReset(ctx context.Context, email string) error {
	// 1. Find user by email
	user, err := s.userRepo.FindByEmail(ctx, email)
	if err != nil {
		// Even if user not found, return success to prevent email enumeration attacks
		log.Printf("Password reset requested for non-existent email: %s", err)
		return nil
	}

	// 2. Gnerate reset token and expiry
	token, err := utils.GenerateSecureToken(32)
	if err != nil {
		return err
	}
	expiresAt := time.Now().Add(15 * time.Minute) // token is valid for 15 minutes
	log.Println("GENERATE RESET PASSWORD TOKEN: ", token)

	// 3. Save token and expiry to user record
	if err := s.userRepo.SetPasswordResetToken(ctx, user.ID, token, expiresAt); err != nil {
		return err
	}

	// 4. Send password reset email
	resetURL := fmt.Sprintf("%s/reset-password?token=%s", s.clientOrigin, token)

	htmlContent, err := s.templateManager.GenerateResetPasswordEmailHTML(emailSvc.TemplateData{
		Name: user.Nickname,
		Link: resetURL,
	})
	if err != nil {
		// Log the error but don't fail the whole signup process
		log.Printf("Failed to generate re-activation email HTML: %v", err)
		return nil
	}

	emailSubject := "[Jingdezhen Ceramics] Reset Your Password"
	plainTextContent := fmt.Sprintf("Please click the following link in 15 minutes to reset your password: %s", resetURL)

	if err := s.emailEnqueuer.EnqueueEmailSend(ctx, jobs.EmailSendPayload{
		To:        email,
		Subject:   emailSubject,
		PlainText: plainTextContent,
		HTML:      htmlContent,
	}); err != nil {
		log.Printf("Failed to enqueue password-reset email to %s: %v", email, err)
	}

	return nil
}

func (s *Service) ResetPassword(ctx context.Context, token string, newPassword string) (*models.AuthResponse, error) {
	// 1. Find user by reset token and check expiry
	// Read and Security Check: verify the token matches AND has not expired
	user, err := s.userRepo.FindByPasswordResetToken(ctx, token)
	if err != nil {
		if errors.Is(err, models.ErrInvalidToken) {
			return nil, models.ErrInvalidToken // Token not found or expired
		}
		return nil, fmt.Errorf("service.ResetPassword.FindToken: %w", err)
	}

	// 2. Hash the new password
	hashedPassword, err := bcrypt.GenerateFromPassword([]byte(newPassword), bcrypt.DefaultCost)
	if err != nil {
		return nil, err
	}

	// 3. Update the user's password and clear the reset token
	// Write and State Change: update users table in database
	if err := s.userRepo.UpdatePasswordAndClearResetToken(ctx, user.ID, string(hashedPassword)); err != nil {
		return nil, err
	}

	// 4. Log the user in by issuing a JWT
	return s.generateAuthResponse(ctx, user)
}

// HandleGoogleLogin generates the redirect URL for the user.
func (s *Service) HandleGoogleLogin() (string, string, error) {
	// Generates the URL the user should be redirected to.
	// The state parameter is crucial for CSRF protection.
	// It should be a random, non-guessable string.
	// In a production app, you'd generate this, store it in a short-lived, secure,
	// HttpOnly cookie, and then compare it in the callback handler.
	state, err := utils.GenerateSecureToken(16)
	if err != nil {
		return "", "", fmt.Errorf("failed to generate state for google login: %w", err)
	}
	// This generates a URL like:
	// https://accounts.google.com/o/oauth2/v2/auth?client_id=...&redirect_uri=...&response_type=code&scope=...&state=...
	url := s.googleOAuthConfig.AuthCodeURL(state)
	return url, state, nil
}

// HandleGoogleCallback processes the callback from Google, completing the login/signup.
func (s *Service) HandleGoogleCallback(ctx context.Context, code string) (*models.AuthResponse, error) {
	// 1. Exchange authorization code for a token from Google
	token, err := s.googleOAuthConfig.Exchange(ctx, code)
	if err != nil {
		return nil, fmt.Errorf("google code exchange failed: %w", err)
	}

	// 2. Use the token to get the user's info from Google's API.
	response, err := http.Get("https://www.googleapis.com/oauth2/v2/userinfo?access_token=" + token.AccessToken)
	if err != nil {
		return nil, fmt.Errorf("failed getting user info from google: %w", err)
	}
	defer response.Body.Close()

	contents, err := io.ReadAll(response.Body)
	if err != nil {
		return nil, fmt.Errorf("failed reading user info response body: %w", err)
	}

	var userInfo GoogleUserInfo
	if err := json.Unmarshal(contents, &userInfo); err != nil {
		return nil, fmt.Errorf("failed to unmarshal user info: %w", err)
	}

	if !userInfo.VerifiedEmail {
		return nil, fmt.Errorf("google email not verified")
	}

	// 3. Find or create user in database
	user, err := s.userRepo.FindByEmail(ctx, userInfo.Email)
	if err != nil && !errors.Is(err, models.ErrNotFound) {
		return nil, fmt.Errorf("db error while finding user by email: %w", err)
	}

	if errors.Is(err, models.ErrNotFound) {
		// User does not exist, create them
		newUser := &models.User{
			Nickname:       userInfo.Name,
			Email:          userInfo.Email,
			AvatarURL:      &userInfo.Picture,
			AuthProvider:   "google",
			AuthProviderID: userInfo.ID,
			IsActive:       true,
		}
		user, err = s.userRepo.CreateOAuthUser(ctx, newUser)
		if err != nil {
			return nil, err
		}
	}
	// If the user was found, you might want to check if their AuthProvider is "email"
	// and potentially link the Google account by setting AuthProvider and AuthProviderID.
	// For now, we'll just log them in.

	// 4. Issue JWT for this user.
	return s.generateAuthResponse(ctx, user)
}

func (s *Service) GetUserProfile(ctx context.Context, userID string) (*models.User, error) {
	user, err := s.userRepo.FindByID(ctx, userID)
	if err != nil {
		return nil, fmt.Errorf("service.GetUserProfile: %w", err)
	}
	roles, err := s.userRepo.GetUserRoles(ctx, userID)
	if err != nil {
		return nil, fmt.Errorf("service.GetUserProfile.Roles: %w", err)
	}
	user.Roles = roles
	return user, nil
}

func (s *Service) UpdateUserProfile(ctx context.Context, userID string, data models.UserUpdateData) (*models.User, error) {
	// Check if nickname is unique if that's a requirement (would need repo method)
	if data.Nickname != nil {
		existingUserWithNickname, err := s.userRepo.FindByNickname(ctx, *data.Nickname)
		if err != nil && !errors.Is(err, models.ErrNotFound) {
			return nil, fmt.Errorf("failed to check nickname uniqueness: %w", err)
		}
		if existingUserWithNickname != nil && existingUserWithNickname.ID != userID {
			return nil, models.ErrNicknameTaken
		}
	}

	updatedUser, err := s.userRepo.Update(ctx, userID, data)
	if err != nil {
		return nil, fmt.Errorf("service.UpdateUserProfile: %w", err)
	}
	return updatedUser, nil
}

func (s *Service) HandleContactSubmission(ctx context.Context, data models.ContactFormData) error {
	// 1. Sanitize inputs
	log.Printf("Contact Form Submitted: Name: %s, Email: %s, Subject: %s",
		data.Name, data.Email, data.Subject)

	emailSubject := fmt.Sprintf("New Contact Form Submission: %s", data.Subject)
	emailBody := fmt.Sprintf(
		"You have received a new message from the contact form:\n\nName: %s\nEmail: %s\n\nMessage:\n%s",
		data.Name, data.Email, data.Message,
	)

	// 2. Enqueue an email to the admin via the job queue (TDD §4.2). A flaky
	// Brevo API should never fail a contact-form submission; the queue retries.
	if err := s.emailEnqueuer.EnqueueEmailSend(ctx, jobs.EmailSendPayload{
		To:        s.adminEmail,
		Subject:   emailSubject,
		PlainText: emailBody,
	}); err != nil {
		log.Printf("Failed to enqueue contact email: %v", err)
		return fmt.Errorf("failed to submit contact message: %w", err)
	}
	log.Printf("Contact email enqueued to %s, Subject: %s", s.adminEmail, emailSubject)

	return nil
}

// --- Admin Service Methods ---
func (s *Service) AdminListUsers(ctx context.Context, page, limit int) ([]models.User, int, error) {
	if page < 1 {
		page = 1
	}
	if limit < 1 || limit > 100 {
		limit = 20
	}
	return s.userRepo.ListAll(ctx, page, limit)
}

// AdminAssignRole sets a user's single staff role (PRD §3.4.1).
// Pass an empty roleKey to clear staff access (return to customer).
func (s *Service) AdminAssignRole(ctx context.Context, targetUserID string, roleKey string) error {
	if roleKey != "" && !models.IsStaffRole(roleKey) {
		return fmt.Errorf("service.AdminAssignRole: invalid role key '%s'", roleKey)
	}
	// Verify the target user exists.
	if _, err := s.userRepo.FindByID(ctx, targetUserID); err != nil {
		return fmt.Errorf("service.AdminAssignRole: target user not found: %w", err)
	}
	if roleKey == "" {
		// Clear staff role: assign then it's a customer. AssignRole currently
		// requires a valid key, so clear directly via a no-op here once a
		// ClearRole repo method exists. For v1, disallow empty (callers must
		// pass a concrete staff role; demoting to customer is a follow-up).
		return fmt.Errorf("service.AdminAssignRole: empty role key not supported yet")
	}
	return s.userRepo.AssignRole(ctx, targetUserID, roleKey)
}
