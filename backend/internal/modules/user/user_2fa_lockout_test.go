package user_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"jingdezhen-ceramics-backend/internal/models"
	"jingdezhen-ceramics-backend/internal/modules/user"
	"jingdezhen-ceramics-backend/internal/testutil"
	"jingdezhen-ceramics-backend/pkg/adapters/ratelimit"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// fakeTwoFAChecker is a controllable TwoFAChecker for the 2FA lockout test.
// ResolvePendingToken always returns the fixed userID; VerifyCodeOrBackup
// returns whatever the test wired in (so the test can drive success/failure).
type fakeTwoFAChecker struct {
	userID    string
	verifyOk  bool
	verifyErr error
}

func (f *fakeTwoFAChecker) IsEnabled(context.Context, string) (bool, error) {
	return true, nil
}
func (f *fakeTwoFAChecker) IssuePendingToken(string, time.Duration) (string, error) {
	return "pending", nil
}
func (f *fakeTwoFAChecker) ResolvePendingToken(string) (string, error) {
	return f.userID, nil
}
func (f *fakeTwoFAChecker) VerifyCodeOrBackup(context.Context, string, string) (bool, error) {
	return f.verifyOk, f.verifyErr
}
func (f *fakeTwoFAChecker) Enroll(context.Context, string, models.EnrollTwoFARequest) (*models.TwoFAEnrollResponse, error) {
	return nil, nil
}
func (f *fakeTwoFAChecker) Confirm(context.Context, string, models.ConfirmTwoFARequest) ([]string, error) {
	return nil, nil
}

// seedLockoutUser inserts an active customer + returns its UUID.
func seedLockoutUser(t *testing.T) (*pgxpool.Pool, string) {
	db := testutil.NewDBPool(t)
	ctx := context.Background()
	var userID string
	err := db.QueryRow(ctx, `
		INSERT INTO users (email, nickname, is_active, auth_provider, password_hash)
		VALUES ('2fa-lockout@jingdezhen.test', 'Lockout Test', true, 'email', 'x')
		RETURNING id::text`).Scan(&userID)
	require.NoError(t, err)
	return db, userID
}

// TestComplete2FALogin_LocksAfterMaxFailures is the priority brute-force
// defense check (TDD §333): after MaxFailures bad codes the account is locked
// — the NEXT attempt returns ErrTooManyAttempts, even with a correct code.
func TestComplete2FALogin_LocksAfterMaxFailures(t *testing.T) {
	if testing.Short() {
		t.Skip("integration test skipped under -short")
	}
	db, userID := seedLockoutUser(t)
	redis := testutil.NewRedisClient(t)
	tracker := ratelimit.NewRedisAttemptTracker(redis)

	svc := user.NewService(
		user.NewRepository(db), nil, nil, // repo, emailEnqueuer, templateManager
		"jwt-secret", "https://test", "admin@test", nil, // jwt, origin, admin, googleOAuth
		&fakeTwoFAChecker{userID: userID, verifyOk: false}, // always-fail verify
		tracker, nil,
	)
	ctx := context.Background()

	// First MaxFailures-1 bad codes → ErrInvalidCredentials, NOT locked.
	for i := 0; i < ratelimit.MaxFailures-1; i++ {
		_, err := svc.Complete2FALogin(ctx, "pending", "bad")
		require.ErrorIs(t, err, models.ErrInvalidCredentials, "attempt %d: bad code, not yet locked", i+1)
	}

	// MaxFailures-th bad code → still ErrInvalidCredentials (the failure that
	// TRIPS the lockout returns the credential error; the lockout bites on the
	// NEXT attempt).
	_, err := svc.Complete2FALogin(ctx, "pending", "bad")
	require.ErrorIs(t, err, models.ErrInvalidCredentials)

	// Now the account is locked — even a CORRECT code returns ErrTooManyAttempts.
	svc2 := user.NewService(
		user.NewRepository(db), nil, nil,
		"jwt-secret", "https://test", "admin@test", nil,
		&fakeTwoFAChecker{userID: userID, verifyOk: true}, // correct code now
		tracker, nil, // SAME tracker — lock state persists across service instances (Redis)
	)
	_, err = svc2.Complete2FALogin(ctx, "pending", "correct")
	require.ErrorIs(t, err, models.ErrTooManyAttempts, "locked: correct code rejected until window expires")

	// Confirm the lock is real in Redis.
	locked, err := tracker.IsLocked(ctx, userID)
	require.NoError(t, err)
	assert.True(t, locked)
}

// TestComplete2FALogin_SuccessResetsCounter pins the reset-on-success seam:
// a user who mistypes a few times then gets it right doesn't carry a
// near-threshold counter forward (a later bad streak starts from 0).
func TestComplete2FALogin_SuccessResetsCounter(t *testing.T) {
	if testing.Short() {
		t.Skip("integration test skipped under -short")
	}
	db, userID := seedLockoutUser(t)
	redis := testutil.NewRedisClient(t)
	tracker := ratelimit.NewRedisAttemptTracker(redis)
	ctx := context.Background()

	// A few failures (under threshold).
	failSvc := user.NewService(
		user.NewRepository(db), nil, nil,
		"jwt-secret", "https://test", "admin@test", nil,
		&fakeTwoFAChecker{userID: userID, verifyOk: false},
		tracker, nil,
	)
	for i := 0; i < ratelimit.MaxFailures-2; i++ {
		_, err := failSvc.Complete2FALogin(ctx, "pending", "bad")
		require.ErrorIs(t, err, models.ErrInvalidCredentials)
	}

	// Then a SUCCESSFUL verify — resets the counter.
	okSvc := user.NewService(
		user.NewRepository(db), nil, nil,
		"jwt-secret", "https://test", "admin@test", nil,
		&fakeTwoFAChecker{userID: userID, verifyOk: true},
		tracker, nil,
	)
	_, err := okSvc.Complete2FALogin(ctx, "pending", "correct")
	require.NoError(t, err)

	// After reset, the same MaxFailures-2 bad streak must NOT lock (counter
	// restarted).
	for i := 0; i < ratelimit.MaxFailures-2; i++ {
		_, err := failSvc.Complete2FALogin(ctx, "pending", "bad")
		require.ErrorIs(t, err, models.ErrInvalidCredentials)
	}
	locked, err := tracker.IsLocked(ctx, userID)
	require.NoError(t, err)
	assert.False(t, locked, "reset cleared the counter; sub-threshold streak doesn't lock")
}

// TestComplete2FALogin_BadPendingTokenShortCircuits: a bad/expired pending
// token returns ErrInvalidToken BEFORE touching the lockout counter (a
// brute-forcer can't inflate a random user's counter without the pending token
// that proves they know the password).
func TestComplete2FALogin_BadPendingTokenShortCircuits(t *testing.T) {
	if testing.Short() {
		t.Skip("integration test skipped under -short")
	}
	db, userID := seedLockoutUser(t)
	redis := testutil.NewRedisClient(t)
	tracker := ratelimit.NewRedisAttemptTracker(redis)
	ctx := context.Background()

	// A TwoFAChecker whose ResolvePendingToken always fails.
	badTokenSvc := user.NewService(
		user.NewRepository(db), nil, nil,
		"jwt-secret", "https://test", "admin@test", nil,
		&badPendingChecker{userID: userID}, // ResolvePendingToken → ErrInvalidToken
		tracker, nil,
	)

	for i := 0; i < ratelimit.MaxFailures+3; i++ {
		_, err := badTokenSvc.Complete2FALogin(ctx, "garbage", "bad")
		require.ErrorIs(t, err, models.ErrInvalidToken)
	}
	// No failures registered → not locked.
	locked, err := tracker.IsLocked(ctx, userID)
	require.NoError(t, err)
	assert.False(t, locked, "bad pending token never reaches the lockout counter")
}

// badPendingChecker's ResolvePendingToken always returns ErrInvalidToken (the
// service maps the error). VerifyCodeOrBackup is unused on this path.
type badPendingChecker struct{ userID string }

func (b *badPendingChecker) IsEnabled(context.Context, string) (bool, error) { return true, nil }
func (b *badPendingChecker) IssuePendingToken(string, time.Duration) (string, error) {
	return "pending", nil
}
func (b *badPendingChecker) ResolvePendingToken(string) (string, error) {
	return "", errors.New("bad token")
}
func (b *badPendingChecker) VerifyCodeOrBackup(context.Context, string, string) (bool, error) {
	return false, nil
}
func (b *badPendingChecker) Enroll(context.Context, string, models.EnrollTwoFARequest) (*models.TwoFAEnrollResponse, error) {
	return nil, nil
}
func (b *badPendingChecker) Confirm(context.Context, string, models.ConfirmTwoFARequest) ([]string, error) {
	return nil, nil
}
