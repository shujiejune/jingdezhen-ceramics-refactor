package twofa

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"jingdezhen-ceramics-backend/internal/models"
	"jingdezhen-ceramics-backend/pkg/utils"
	"log"
	"strings"
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
	// Confirm verifies the first TOTP code against the staged secret, enables
	// 2FA, and generates the one-time backup codes (returned in plaintext ONCE).
	Confirm(ctx context.Context, userID string, req models.ConfirmTwoFARequest) ([]string, error)
	// Disable turns 2FA off for a user (keeps the secret; re-enroll to re-stage).
	Disable(ctx context.Context, userID string) error
	// IsEnabled returns whether the user has a confirmed, enabled 2FA record.
	IsEnabled(ctx context.Context, userID string) (bool, error)
	// VerifyCode checks a 6-digit code against the user's stored (decrypted)
	// TOTP secret only. Used by the enrollment confirm step (the first code).
	VerifyCode(ctx context.Context, userID, code string) (bool, error)
	// VerifyCodeOrBackup checks the TOTP code first; if it fails, tries to
	// consume a one-time backup code. Used by the login-verify step so a user
	// who lost their authenticator can still log in with a backup code.
	VerifyCodeOrBackup(ctx context.Context, userID, code string) (bool, error)
	// RegenerateBackupCodes invalidates the user's remaining unused backup codes
	// and issues a fresh set (returned in plaintext ONCE).
	RegenerateBackupCodes(ctx context.Context, userID string) ([]string, error)
	// CountUnusedBackupCodes reports how many backup codes remain.
	CountUnusedBackupCodes(ctx context.Context, userID string) (int, error)
	// IssuePendingToken returns a short-lived JWT identifying a user who has
	// passed the password check but still owes a TOTP code (or must enroll).
	// The login flow uses it when 2FA is enabled; the frontend POSTs it + the
	// code to /auth/2fa/verify. `ttl` is the token's lifetime (verify step is
	// fast → 5m; the must-enroll step needs QR scan time → 15m).
	IssuePendingToken(userID string, ttl time.Duration) (string, error)
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

func (s *Service) Confirm(ctx context.Context, userID string, req models.ConfirmTwoFARequest) ([]string, error) {
	// Load the staged (unconfirmed) record to decrypt the secret.
	rec, err := s.repo.Get(ctx, userID)
	if err != nil {
		if errors.Is(err, models.ErrNotFound) {
			return nil, models.ErrNotFound
		}
		return nil, fmt.Errorf("twofa.Confirm: %w", err)
	}
	secret, err := utils.Decrypt(s.encKey, rec.EncryptedSecret)
	if err != nil {
		return nil, fmt.Errorf("twofa.Confirm: decrypt: %w", err)
	}
	if !totp.Validate(req.Code, string(secret)) {
		return nil, models.ErrInvalidCredentials
	}
	if err := s.repo.Confirm(ctx, userID); err != nil {
		return nil, fmt.Errorf("twofa.Confirm: %w", err)
	}
	// 2FA is now enabled — generate the one-time backup codes (recovery path for
	// a lost authenticator). Stored hashed; returned in plaintext ONCE.
	codes, hashes, err := generateBackupCodes(s.encKey, numBackupCodes)
	if err != nil {
		return nil, fmt.Errorf("twofa.Confirm: generate backup codes: %w", err)
	}
	if err := s.repo.StoreBackupCodes(ctx, userID, hashes); err != nil {
		return nil, fmt.Errorf("twofa.Confirm: store backup codes: %w", err)
	}
	log.Printf("2FA confirmed and enabled for user %s (%d backup codes issued)", userID, len(codes))
	return codes, nil
}

func (s *Service) Disable(ctx context.Context, userID string) error {
	if err := s.repo.Disable(ctx, userID); err != nil {
		return fmt.Errorf("twofa.Disable: %w", err)
	}
	// Also drop backup codes so a later re-enrollment can't revive stale codes.
	if err := s.repo.DeleteBackupCodes(ctx, userID); err != nil {
		return fmt.Errorf("twofa.Disable: delete backup codes: %w", err)
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

// VerifyCodeOrBackup is the login-verify credential check. It tries the TOTP
// code first (stateless; TOTP codes are reusable within their time window, which
// is standard). If that fails it tries to consume a one-time backup code.
// Only one path may succeed; a backup code is single-use (marked used on match).
func (s *Service) VerifyCodeOrBackup(ctx context.Context, userID, code string) (bool, error) {
	ok, err := s.VerifyCode(ctx, userID, code)
	if err != nil {
		return false, err
	}
	if ok {
		return true, nil
	}
	// TOTP failed — try a backup code (normalized before hashing).
	hash := hashBackupCode(s.encKey, code)
	return s.repo.ConsumeBackupCode(ctx, userID, hash)
}

func (s *Service) RegenerateBackupCodes(ctx context.Context, userID string) ([]string, error) {
	// Only users with 2FA enabled should regenerate (otherwise we'd mint codes
	// for a user whose 2FA is off — pointless and a confusion vector).
	enabled, err := s.IsEnabled(ctx, userID)
	if err != nil {
		return nil, fmt.Errorf("twofa.RegenerateBackupCodes: %w", err)
	}
	if !enabled {
		return nil, models.ErrInvalidOperation
	}
	codes, hashes, err := generateBackupCodes(s.encKey, numBackupCodes)
	if err != nil {
		return nil, fmt.Errorf("twofa.RegenerateBackupCodes: generate: %w", err)
	}
	if err := s.repo.StoreBackupCodes(ctx, userID, hashes); err != nil {
		return nil, fmt.Errorf("twofa.RegenerateBackupCodes: store: %w", err)
	}
	log.Printf("2FA backup codes regenerated for user %s", userID)
	return codes, nil
}

func (s *Service) CountUnusedBackupCodes(ctx context.Context, userID string) (int, error) {
	n, err := s.repo.CountUnusedBackupCodes(ctx, userID)
	if err != nil {
		return 0, fmt.Errorf("twofa.CountUnusedBackupCodes: %w", err)
	}
	return n, nil
}

// --- Backup-code generation + hashing ---------------------------------------

// numBackupCodes is how many one-time codes are issued per generation.
const numBackupCodes = 8

// backupAlphabet excludes ambiguous chars (no 0/O/1/I/L) for safe transcription.
const backupAlphabet = "ABCDEFGHJKLMNPQRSTUVWXYZ23456789"

// generateBackupCodes produces n random one-time codes (displayed XXXX-XXXX)
// and their hashes. The internal/normalized form (8 uppercase alphanumerics,
// dash-stripped) is what gets hashed; users may enter the code with or without
// the dash and in any case (verify normalizes before hashing).
func generateBackupCodes(pepper []byte, n int) (codes []string, hashes []string, err error) {
	codes = make([]string, 0, n)
	hashes = make([]string, 0, n)
	for i := 0; i < n; i++ {
		buf := make([]byte, 8)
		if _, err := rand.Read(buf); err != nil {
			return nil, nil, fmt.Errorf("generateBackupCodes: %w", err)
		}
		var b strings.Builder
		for _, c := range buf {
			b.WriteByte(backupAlphabet[int(c)%len(backupAlphabet)])
		}
		norm := b.String()
		codes = append(codes, norm[:4]+"-"+norm[4:]) // display form
		hashes = append(hashes, hashBackupCode(pepper, norm))
	}
	return codes, hashes, nil
}

// hashBackupCode returns the SHA-256 hex of (pepper || normalized code). The
// pepper is the 2FA encryption key (reused as a hash pepper so a DB leak alone
// — without the app key — yields useless hashes).
func hashBackupCode(pepper []byte, code string) string {
	norm := strings.ToUpper(strings.Map(func(r rune) rune {
		if r == '-' || r == ' ' {
			return -1 // drop
		}
		return r
	}, code))
	h := sha256.New()
	if pepper != nil {
		h.Write(pepper)
	}
	h.Write([]byte(norm))
	return hex.EncodeToString(h.Sum(nil))
}

// pending2FAClaims is a short-lived JWT (5 min) whose only purpose is to
// identify a user who passed the password check but must still supply a TOTP
// code. It carries no roles and grants no access except to the 2FA-verify step.
type pending2FAClaims struct {
	UserID string `json:"user_id"`
	jwt.RegisteredClaims
}

func (s *Service) IssuePendingToken(userID string, ttl time.Duration) (string, error) {
	claims := pending2FAClaims{
		UserID: userID,
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(ttl)),
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
