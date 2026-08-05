package models

import "errors"

var ErrNotFound = errors.New("requested resource not found")
var ErrForbidden = errors.New("user does not have permission to access this resource")
var ErrConflict = errors.New("resource conflict, item already exists")
var ErrInactiveAccount = errors.New("user account is not active")
var ErrInvalidToken = errors.New("token not found or expired")
var ErrInvalidCredentials = errors.New("invalid credentials")                    // email or password provided does not match database record
var ErrTooManyAttempts = errors.New("too many failed attempts, try again later") // 2FA brute-force lockout (TDD §333)
var ErrInvalidForumPostCategoryID = errors.New("invalid category of forum post")
var ErrInvalidOperation = errors.New("the requested operation is not valid for the target resource")
var ErrLimitExceeded = errors.New("submission or usage limit exceeded")
var ErrMissedDeadline = errors.New("deadline missed")
var ErrNicknameTaken = errors.New("nickname already taken")
var ErrNotOwned = errors.New("not owned by this user")
var Err2FARequired = errors.New("two-factor authentication code required")  // password OK but a TOTP code is needed to complete login
var Err2FAEnrollmentRequired = errors.New("two-factor enrollment required") // super_admin must enroll 2FA before a full session is granted
var ErrAccountDeleted = errors.New("user account has been deleted")         // GDPR erasure: login rejected for an anonymized stub
var ErrRateNotFound = errors.New("fx: rate not found for currency")         // no fx_rates row exists for the requested currency (refresh not yet run)
var ErrInvalidLocale = errors.New("unsupported locale")
var ErrInvalidWorkflowTransition = errors.New("invalid content workflow transition for this actor")
var ErrCartEmpty = errors.New("cart is empty; cannot check out")                                // checkout on an empty cart (PRD §3.2.3)
var ErrUnshippable = errors.New("destination country is not shippable")                         // no shipping_fee_tiers row for the country (PRD §3.2.3)
var ErrOverweight = errors.New("order exceeds the maximum shipping weight for the destination") // PRD §3.2.3 overweight block
var ErrWebhookSignatureInvalid = errors.New("webhook signature verification failed")            // gateway webhook signature mismatch (TDD §10)
var ErrPaymentNotSucceeded = errors.New("no succeeded payment found for the order")             // refund on an unpaid order
var ErrGatewayUnavailable = errors.New("payment gateway is not configured")                     // PAYMENTS_MODE live but no creds / unknown gateway

// Itinerary quote + deposit (PRD §3.3.2, TDD §3.4 M3 #3).
var ErrRequestNotQuoted = errors.New("itinerary request is not in a quoted state")
var ErrQuoteNotPayable = errors.New("quote is not in a payable state")                         // not 'sent' at pay time
var ErrQuoteAlreadyPaid = errors.New("deposit has already been paid for this quote")           // replayed pay
var ErrInvalidQuote = errors.New("quote line items are invalid")                               // unknown option_key / qty mismatch
var ErrConsentRequired = errors.New("privacy policy consent is required")                      // PRD §3.3.2 step 4: the GDPR checkbox must be checked to submit
var ErrItineraryNotCancellable = errors.New("itinerary request is not in a cancellable state") // only `pending` is customer-cancellable

// Analytics (TDD §3.4/§4.2, PRD §3.4.2). The consent gate returns this when the
// visitor has not granted cookie_analytics consent — the handler maps it to 204
// (event silently dropped, not a client error).
var ErrConsentNotGranted = errors.New("analytics consent not granted")
