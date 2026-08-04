package payment

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"jingdezhen-ceramics-backend/internal/models"
	"jingdezhen-ceramics-backend/pkg/adapters/payments"

	"github.com/shopspring/decimal"
)

// --- Injected dependencies (narrow interfaces) ---

// OrderFinalizer drives the order created→paid transition. Implemented by
// order.Service; kept as an interface so the payment module doesn't import
// the order module (avoids an import cycle: order imports payment).
type OrderFinalizer interface {
	MarkPaid(ctx context.Context, orderID int64) error
}

// OrderLoader loads an order for refund (to read the presentment amount +
// currency). Implemented by order.Service.GetAdmin.
type OrderLoader interface {
	GetAdmin(ctx context.Context, orderID int64) (*models.Order, error)
}

// PaymentEnqueuer enqueues the payment:finalize job (the webhook acks 200
// immediately; the worker finalizes the order) + the itinerary-deposit variant.
// Implemented by jobs.Client.
type PaymentEnqueuer interface {
	EnqueuePaymentFinalize(ctx context.Context, orderID int64, success bool, gateway, gatewayRef string) error
	EnqueueItineraryDepositFinalize(ctx context.Context, quoteID int64, success bool, gateway, gatewayRef string) error
}

// GatewayRegistry resolves a Gateway by name (the webhook path needs this).
type GatewayRegistry interface {
	Get(name string) (payments.Gateway, error)
}

// ServiceInterface defines payment business logic.
type ServiceInterface interface {
	// CreateIntent creates a gateway payment intent for an order + records the
	// pending payment row. Returns the hosted checkout URL.
	CreateIntent(ctx context.Context, gatewayName string, orderID int64, amountMinor int64, currency string) (string, error)
	// CreateQuoteIntent creates a gateway intent for an itinerary deposit +
	// records the pending payment row (itinerary_quote_id, order_id NULL).
	// Mirrors CreateIntent for the deposit path (TDD §3.4 line 189).
	CreateQuoteIntent(ctx context.Context, gatewayName string, quoteID int64, amountMinor int64, currency string) (string, error)
	// HandleWebhook verifies a gateway webhook signature, idempotently records
	// the event, and (on first-seen success) enqueues payment:finalize. The
	// HTTP handler acks 200 immediately.
	HandleWebhook(ctx context.Context, gatewayName string, rawBody []byte, headers map[string]string) error
	// Refund issues a full refund for an order's succeeded payment via the
	// gateway, marks the payment refunded, and transitions the order refunded.
	// Returns ErrPaymentNotSucceeded if no succeeded payment exists.
	Refund(ctx context.Context, orderID int64, reason string) error
	// RefundQuote issues a full refund for a quote's succeeded deposit via the
	// gateway + marks the payment refunded. Fail-closed: gateway first.
	RefundQuote(ctx context.Context, quoteID int64, reason string) error
}

type Service struct {
	repo      RepositoryInterface
	registry  GatewayRegistry
	finalizer OrderFinalizer
	loader    OrderLoader
	enqueuer  PaymentEnqueuer
	returnURL string // where the gateway redirects after payment (PRD §3.2.3)
}

func NewService(
	repo RepositoryInterface,
	registry GatewayRegistry,
	finalizer OrderFinalizer,
	loader OrderLoader,
	enqueuer PaymentEnqueuer,
	returnURL string,
) ServiceInterface {
	return &Service{repo: repo, registry: registry, finalizer: finalizer, loader: loader, enqueuer: enqueuer, returnURL: returnURL}
}

// CreateIntent creates a gateway intent + records the pending payment row.
// The order is already created (status=created, stock decremented) by the
// time this is called; a gateway error leaves a cancellable `created` order.
func (s *Service) CreateIntent(ctx context.Context, gatewayName string, orderID int64, amountMinor int64, currency string) (string, error) {
	gw, err := s.registry.Get(gatewayName)
	if err != nil {
		return "", models.ErrGatewayUnavailable
	}
	resp, err := gw.CreateIntent(ctx, payments.IntentRequest{
		OrderID: orderID, AmountMinor: amountMinor, Currency: currency,
		Reference: fmt.Sprintf("JDZ-#%d", orderID), ReturnURL: s.returnURL,
	})
	if err != nil {
		return "", fmt.Errorf("payment.CreateIntent: %w", err)
	}
	p := &models.Payment{
		OrderID: orderID, Gateway: gw.Name(), GatewayRef: resp.GatewayRef,
		Status: models.PaymentPending, AmountMinor: amountMinor, Currency: currency,
		IdempotencyKey: resp.GatewayRef + ":intent",
	}
	if _, err := s.repo.RecordIntent(ctx, p); err != nil {
		return "", fmt.Errorf("payment.CreateIntent.RecordIntent: %w", err)
	}
	return resp.HostedURL, nil
}

// HandleWebhook verifies the gateway signature, idempotently records the
// event, and — on a first-seen succeeded event — enqueues payment:finalize
// (the worker drives the order created→paid). A replayed webhook is a 200
// no-op (idempotency_key UNIQUE, TDD §11).
func (s *Service) HandleWebhook(ctx context.Context, gatewayName string, rawBody []byte, headers map[string]string) error {
	gw, err := s.registry.Get(gatewayName)
	if err != nil {
		return models.ErrGatewayUnavailable
	}
	ev, err := gw.VerifyWebhook(ctx, rawBody, headers)
	if err != nil {
		if errors.Is(err, payments.ErrWebhookSignatureInvalid) {
			return models.ErrWebhookSignatureInvalid
		}
		return fmt.Errorf("payment.HandleWebhook.Verify: %w", err)
	}
	// Entity resolution: the webhook event may carry OrderID (Airwallex/mock for
	// orders) or neither (PayPal capture, or a deposit where the mock carries the
	// quote id as OrderID — ambiguous). Resolve by gateway_ref when OrderID is 0:
	// the pending intent row was keyed by gateway_ref at CreateIntent/CreateQuoteIntent.
	var quoteID int64
	if ev.OrderID == 0 {
		if p, perr := s.repo.GetByGatewayRef(ctx, ev.GatewayRef); perr == nil {
			ev.OrderID = p.OrderID // 0 for a deposit; the enqueue dispatches on quoteID
			quoteID = p.ItineraryQuoteID
		}
	}
	p := &models.Payment{
		OrderID: ev.OrderID, ItineraryQuoteID: quoteID, Gateway: gw.Name(),
		GatewayRef: ev.GatewayRef, Status: webhookStatusToPayment(ev.Status),
		AmountMinor: ev.AmountMinor, Currency: ev.Currency,
		RawWebhook: ev.Raw, IdempotencyKey: ev.IdempotencyKey,
	}
	inserted, err := s.repo.UpsertWebhook(ctx, p)
	if err != nil {
		return fmt.Errorf("payment.HandleWebhook.Upsert: %w", err)
	}
	if !inserted {
		// A replayed webhook — already seen. Ack as success (idempotent).
		return nil
	}
	if ev.Status == payments.WebhookSucceeded {
		if quoteID != 0 {
			if err := s.enqueuer.EnqueueItineraryDepositFinalize(ctx, quoteID, true, gw.Name(), ev.GatewayRef); err != nil {
				log.Printf("payment.HandleWebhook.Enqueue(quote=%d): %v (manual finalize needed)", quoteID, err)
			}
		} else {
			if err := s.enqueuer.EnqueuePaymentFinalize(ctx, ev.OrderID, true, gw.Name(), ev.GatewayRef); err != nil {
				log.Printf("payment.HandleWebhook.Enqueue(order=%d): %v (manual finalize needed)", ev.OrderID, err)
			}
		}
	}
	return nil
}

// Refund issues a full refund for the order's succeeded payment via the
// gateway, marks the payment refunded, and transitions the order refunded.
// Fail-closed: a gateway error leaves the order paid (no transition).
func (s *Service) Refund(ctx context.Context, orderID int64, reason string) error {
	p, err := s.repo.GetSucceededByOrderID(ctx, orderID)
	if err != nil {
		if errors.Is(err, models.ErrNotFound) {
			return models.ErrPaymentNotSucceeded
		}
		return err
	}
	gw, err := s.registry.Get(p.Gateway)
	if err != nil {
		return models.ErrGatewayUnavailable
	}
	if _, err := gw.Refund(ctx, payments.RefundRequest{
		GatewayRef: p.GatewayRef, AmountMinor: p.AmountMinor, Currency: p.Currency, Reason: reason,
	}); err != nil {
		return fmt.Errorf("payment.Refund.gateway: %w", err)
	}
	if err := s.repo.SetRefunded(ctx, p.ID); err != nil {
		return err
	}
	return nil
}

// CreateQuoteIntent creates a gateway intent for an itinerary deposit +
// records the pending payment row (itinerary_quote_id set, order_id NULL).
// Mirrors CreateIntent for the deposit path (TDD §3.4 line 189). The quoteID
// is passed as IntentRequest.OrderID so the mock gateway builds a stable ref
// ("mock-<quoteID>"); live gateways would use a merchant reference instead.
func (s *Service) CreateQuoteIntent(ctx context.Context, gatewayName string, quoteID int64, amountMinor int64, currency string) (string, error) {
	gw, err := s.registry.Get(gatewayName)
	if err != nil {
		return "", models.ErrGatewayUnavailable
	}
	resp, err := gw.CreateIntent(ctx, payments.IntentRequest{
		OrderID: quoteID, AmountMinor: amountMinor, Currency: currency,
		Reference: fmt.Sprintf("JDZ-ITIN-#%d", quoteID), ReturnURL: s.returnURL,
	})
	if err != nil {
		return "", fmt.Errorf("payment.CreateQuoteIntent: %w", err)
	}
	p := &models.Payment{
		ItineraryQuoteID: quoteID, Gateway: gw.Name(), GatewayRef: resp.GatewayRef,
		Status: models.PaymentPending, AmountMinor: amountMinor, Currency: currency,
		IdempotencyKey: resp.GatewayRef + ":intent",
	}
	if _, err := s.repo.RecordIntent(ctx, p); err != nil {
		return "", fmt.Errorf("payment.CreateQuoteIntent.RecordIntent: %w", err)
	}
	return resp.HostedURL, nil
}

// RefundQuote issues a full refund for a quote's succeeded deposit via the
// gateway + marks the payment refunded. Fail-closed: gateway first; a gateway
// error leaves the quote paid (the itinerary service does NOT cancel the quote
// until this returns nil). Mirrors Refund.
func (s *Service) RefundQuote(ctx context.Context, quoteID int64, reason string) error {
	p, err := s.repo.GetSucceededByItineraryQuoteID(ctx, quoteID)
	if err != nil {
		if errors.Is(err, models.ErrNotFound) {
			return models.ErrPaymentNotSucceeded
		}
		return err
	}
	gw, err := s.registry.Get(p.Gateway)
	if err != nil {
		return models.ErrGatewayUnavailable
	}
	if _, err := gw.Refund(ctx, payments.RefundRequest{
		GatewayRef: p.GatewayRef, AmountMinor: p.AmountMinor, Currency: p.Currency, Reason: reason,
	}); err != nil {
		return fmt.Errorf("payment.RefundQuote.gateway: %w", err)
	}
	return s.repo.SetRefunded(ctx, p.ID)
}

func webhookStatusToPayment(s payments.WebhookStatus) models.PaymentStatus {
	if s == payments.WebhookSucceeded {
		return models.PaymentSucceeded
	}
	return models.PaymentFailed
}

// (compile-time: keep json + decimal imported; used by webhook raw capture
// + future amount parsing helpers)
var _ = json.RawMessage(nil)
var _ = decimal.Zero
