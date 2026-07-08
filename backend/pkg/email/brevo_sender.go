package email

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"time"
)

// BrevoSender implements ServiceInterface using Brevo's REST API (TDD §10).
// Replaces the former AWS SES sender. Brevo has no heavyweight SDK requirement:
// a single POST to /v3/smtp/email with an `api-key` header is all that's needed,
// so we keep zero extra Go dependencies here.
type BrevoSender struct {
	apiKey      string
	senderEmail string
	senderName  string
	client      *http.Client // reused for pooled connections + sane timeouts (TDD §11.1.6)
}

// NewBrevoSender constructs a Brevo email sender.
// In sandbox/dev, pass an empty apiKey to get a no-op sender that logs the
// email instead of calling the live API (so local dev never needs real keys).
func NewBrevoSender(apiKey, senderEmail, senderName string) *BrevoSender {
	return &BrevoSender{
		apiKey:      apiKey,
		senderEmail: senderEmail,
		senderName:  senderName,
		client:      &http.Client{Timeout: 15 * time.Second},
	}
}

const brevoEndpoint = "https://api.brevo.com/v3/smtp/email"

// brevoEmailRequest is the JSON body for Brevo's transactional email API.
type brevoEmailRequest struct {
	Sender struct {
		Email string `json:"email"`
		Name  string `json:"name,omitempty"`
	} `json:"sender"`
	To []struct {
		Email string `json:"email"`
	} `json:"to"`
	Subject      string `json:"subject"`
	HTMLContent  string `json:"htmlContent"`
	TextContent  string `json:"textContent,omitempty"`
}

// SendEmail sends a transactional email via Brevo. If no API key is configured
// (local dev / sandbox without keys), it logs the message instead of calling
// the network — so the app runs without real Brevo credentials locally.
func (s *BrevoSender) SendEmail(ctx context.Context, to, subject, plainTextContent, htmlContent string) error {
	if s.apiKey == "" {
		log.Printf("[brevo-noop] to=%s subject=%q", to, subject)
		return nil
	}

	body := brevoEmailRequest{
		Subject:     subject,
		HTMLContent: htmlContent,
		TextContent: plainTextContent,
	}
	body.Sender.Email = s.senderEmail
	body.Sender.Name = s.senderName
	body.To = []struct {
		Email string `json:"email"`
	}{{Email: to}}

	payload, err := json.Marshal(body)
	if err != nil {
		return fmt.Errorf("brevo sender: marshal: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, brevoEndpoint, bytes.NewReader(payload))
	if err != nil {
		return fmt.Errorf("brevo sender: new request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("api-key", s.apiKey)
	req.Header.Set("accept", "application/json")

	resp, err := s.client.Do(req)
	if err != nil {
		return fmt.Errorf("brevo sender: http: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		respBody, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("brevo sender: API returned %d: %s", resp.StatusCode, string(respBody))
	}

	log.Printf("Successfully sent email to %s via Brevo", to)
	return nil
}
