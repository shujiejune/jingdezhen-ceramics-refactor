package email

import "context"

// ServiceInterface is the contract every email adapter (Brevo now, SES before)
// must satisfy. Services depend on this interface, never on a concrete sender,
// so swapping providers is an env-var flip (TDD §4.1, §10).
type ServiceInterface interface {
	SendEmail(ctx context.Context, to, subject, plainTextContent, htmlContent string) error
}
