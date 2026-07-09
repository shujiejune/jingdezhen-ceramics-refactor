package twofa

import (
	"context"
	"errors"
	"fmt"
	"jingdezhen-ceramics-backend/internal/models"
	"jingdezhen-ceramics-backend/pkg/utils"
	"log"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/pquerna/otp/totp"
)

// ServiceInterface defines TOTP 2FA business logic.
type ServiceInterface interface {
	// Enroll starts 2FA enrollment: generates a TOTP secret, stores it encrypted
	// (unconfirmed), and returns the otpauth:// URI for the QR code. The raw
	// secret is shown once here; only the encrypted form is persisted.
	Enroll(ctx context.Context, userID string, req models.EnrollTwoFARequest) (*models.TwoFAEnrollResponse, error)
	// Confirm verifies the first 6-digit code against the staged secret and, on
	// success, marks 2FA as enabled+confirmed.
	Confirm(ctx context.Context, userID string, req models.ConfirmTwoFARequest) error
	// Disable turns 2FA off for a user (keeps the secret; re-enroll to re-stage).
	Disable(ctx context.Context, userID string) error
	// IsEnabled returns whether the user has a confirmed, enabled 2FA record.
	IsEnabled(ctx context.Context, userID string) (bool, error)
	// VerifyCode checks a 6-digit code against the user's stored (decrypted)
	// TOTP secret. Used by the login 2FA-verify step.
	VerifyCode(ctx context.Context, userID, code string) (bool, error)
	// IssuePendingToken returns a short-lived JWT identifying a user who has
	// passed the password check but still owes a TOTP code. The login flow uses
	// it when 2FA is enabled; the frontend POSTs it + the code to /auth/2fa/verify.
	IssuePendingToken(userID string) (string, error)
	// ResolvePendingToken validates a pending 2FA token and returns the userID.
	// Called by the verify endpoint before checking the code.
	ResolvePendingToken(token string) (string, error)
}

type Service struct {
	repo        RepositoryInterface
	encKey      []byte // app key for encrypting the TOTP secret at rest
	jwtSecret   string // for signing the pending 2FA token
	issuer      string // default issuer label for otpauth URIs
}

func NewService(repo RepositoryInterface, encKey []byte, jwtSecret, issuer string) ServiceInterface {
	if issuer == "" {
		issuer = "Jingdezhen Ceramics"
	}
	return &Service{repo: repo, encKey: encKey, jwtSecret: jwtSecret, issuer: issuer}
}

func (s *Service) Enroll(ctx context.Context, userID string, req models.EnrollTwoFARequest) (*models.TwoFAEnrollResponse, error) {
	account := req.Account
	if account == "" {
		account = userID
	}
	issuer := req.Issuer
	if issuer == "" {
		issuer = s.issuer
	}

	key, err := totp.Generate(totp.GenerateOpts{
		Issuer:      issuer,
		AccountName: account,
	})
	if err != nil {
		return nil, fmt.Errorf("twofa.Enroll: generate key: %w", err)
	}

	encSecret, err := utils.Encrypt(s.encKey, []byte(key.Secret()))
	if err != nil {
		return nil, fmt.Errorf("twofa.Enroll: encrypt secret: %w", err)
	}
	if err := s.repo.UpsertStage(ctx, userID, encSecret); err != nil {
		return nil, fmt.Errorf("twofa.Enroll: %w", err)
	}
	log.Printf("2FA enrollment staged for user %s", userID)
	return &models.TwoFAEnrollResponse{
		OTPAuthURI: key.URL(),
		Secret:     key.Secret(),
	}, nil
}

func (s *Service) Confirm(ctx context.Context, userID string, req models.ConfirmTwoFARequest) error {
	// Load the staged (unconfirmed) record to decrypt the secret.
	rec, err := s.repo.Get(ctx, userID)
	if err != nil {
		if errors.Is(err, models.ErrNotFound) {
			return models.ErrNotFound
		}
		return fmt.Errorf("twofa.Confirm: %w", err)
	}
	secret, err := utils.Decrypt(s.encKey, rec.EncryptedSecret)
	if err != nil {
		return fmt.Errorf("twofa.Confirm: decrypt: %w", err)
	}
	if !totp.Validate(req.Code, string(secret)) {
		return models.ErrInvalidCredentials
	}
	if err := s.repo.Confirm(ctx, userID); err != nil {
		return fmt.Errorf("twofa.Confirm: %w", err)
	}
	log.Printf("2FA confirmed and enabled for user %s", userID)
	return nil
}

func (s *Service) Disable(ctx context.Context, userID string) error {
	if err := s.repo.Disable(ctx, userID); err != nil {
		return fmt.Errorf("twofa.Disable: %w", err)
	}
	log.Printf("2FA disabled for user %s", userID)
	return nil
}

func (s *Service) IsEnabled(ctx context.Context, userID string) (bool, error) {
	rec, err := s.repo.Get(ctx, userID)
	if err != nil {
		if errors.Is(err, models.ErrNotFound) {
			return false, nil // no record = not enabled (not an error)
		}
		return false, fmt.Errorf("twofa.IsEnabled: %w", err)
	}
	return rec.Enabled, nil
}

func (s *Service) VerifyCode(ctx context.Context, userID, code string) (bool, error) {
	rec, err := s.repo.Get(ctx, userID)
	if err != nil {
		if errors.Is(err, models.ErrNotFound) {
			return false, nil
		}
		return false, fmt.Errorf("twofa.VerifyCode: %w", err)
	}
	if !rec.Enabled {
		return false, nil
	}
	secret, err := utils.Decrypt(s.encKey, rec.EncryptedSecret)
	if err != nil {
		return false, fmt.Errorf("twofa.VerifyCode: decrypt: %w", err)
	}
	return totp.Validate(code, string(secret)), nil
}

// pending2FAClaims is a short-lived JWT (5 min) whose only purpose is to
// identify a user who passed the password check but must still supply a TOTP
// code. It carries no roles and grants no access except to the 2FA-verify step.
type pending2FAClaims struct {
	UserID string `json:"user_id"`
	jwt.RegisteredClaims
}

func (s *Service) IssuePendingToken(userID string) (string, error) {
	claims := pending2FAClaims{
		UserID: userID,
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(5 * time.Minute)),
			Subject:   "2fa-pending",
		},
	}
	tok := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	signed, err := tok.SignedString([]byte(s.jwtSecret))
	if err != nil {
		return "", fmt.Errorf("twofa.IssuePendingToken: %w", err)
	}
	return signed, nil
}

func (s *Service) ResolvePendingToken(token string) (string, error) {
	claims := pending2FAClaims{}
	_, err := jwt.ParseWithClaims(token, &claims, func(t *jwt.Token) (any, error) {
		if _, ok := t.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, fmt.Errorf("unexpected signing method: %v", t.Header["alg"])
		}
		return []byte(s.jwtSecret), nil
	})
	if err != nil {
		return "", models.ErrInvalidToken
	}
	return claims.UserID, nil
}
