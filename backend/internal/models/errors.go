package models

import "errors"

var ErrNotFound = errors.New("requested resource not found")
var ErrForbidden = errors.New("user does not have permission to access this resource")
var ErrConflict = errors.New("resource conflict, item already exists")
var ErrInactiveAccount = errors.New("user account is not active")
var ErrInvalidToken = errors.New("token not found or expired")
var ErrInvalidCredentials = errors.New("invalid credentials") // email or password provided does not match database record
var ErrInvalidForumPostCategoryID = errors.New("invalid category of forum post")
var ErrInvalidOperation = errors.New("the requested operation is not valid for the target resource")
var ErrLimitExceeded = errors.New("submission or usage limit exceeded")
var ErrMissedDeadline = errors.New("deadline missed")
var ErrNicknameTaken = errors.New("nickname already taken")
var ErrNotOwned = errors.New("not owned by this user")
var Err2FARequired = errors.New("two-factor authentication code required")          // password OK but a TOTP code is needed to complete login
var Err2FAEnrollmentRequired = errors.New("two-factor enrollment required") // super_admin must enroll 2FA before a full session is granted
var ErrAccountDeleted = errors.New("user account has been deleted")          // GDPR erasure: login rejected for an anonymized stub
var ErrRateNotFound = errors.New("fx: rate not found for currency")         // no fx_rates row exists for the requested currency (refresh not yet run)
var ErrInvalidLocale = errors.New("unsupported locale")
var ErrInvalidWorkflowTransition = errors.New("invalid content workflow transition for this actor")
