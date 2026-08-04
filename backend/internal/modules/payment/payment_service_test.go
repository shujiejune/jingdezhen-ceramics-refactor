package payment

import (
	"context"
	"encoding/json"
	"sync"
	"testing"

	"jingdezhen-ceramics-backend/internal/models"
	"jingdezhen-ceramics-backend/pkg/adapters/payments"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// fakeRepo is an in-memory Repository for testing the webhook idempotency
// logic without a database (TDD §11 priority: webhook idempotency).
type fakeRepo struct {
	mu     sync.Mutex
	events map[string]*models.Payment // keyed by idempotency_key
	intent map[string]*models.Payment // keyed by gateway_ref (the pending intent)
}

func newFakeRepo() *fakeRepo {
	return &fakeRepo{events: map[string]*models.Payment{}, intent: map[string]*models.Payment{}}
}

func (f *fakeRepo) RecordIntent(_ context.Context, p *models.Payment) (int64, error) {
	f.mu.Lock(); defer f.mu.Unlock()
	f.intent[p.GatewayRef] = p
	f.events[p.IdempotencyKey] = p
	return int64(len(f.events)), nil
}
func (f *fakeRepo) GetByID(context.Context, int64) (*models.Payment, error) { return nil, models.ErrNotFound }
func (f *fakeRepo) GetSucceededByOrderID(context.Context, int64) (*models.Payment, error) {
	return nil, models.ErrNotFound
}
func (f *fakeRepo) UpsertWebhook(_ context.Context, p *models.Payment) (bool, error) {
	f.mu.Lock(); defer f.mu.Unlock()
	if _, exists := f.events[p.IdempotencyKey]; exists {
		return false, nil // replay → no-op
	}
	f.events[p.IdempotencyKey] = p
	if intent, ok := f.intent[p.GatewayRef]; ok {
		intent.Status = p.Status
	}
	return true, nil
}
func (f *fakeRepo) MarkStatus(context.Context, int64, models.PaymentStatus, models.PaymentStatus) error { return nil }
func (f *fakeRepo) SetRefunded(context.Context, int64) error { return nil }
func (f *fakeRepo) GetByGatewayRef(context.Context, string) (*models.Payment, error) { return nil, models.ErrNotFound }
func (f *fakeRepo) MarkSucceededByGatewayRef(context.Context, string) error { return nil }
func (f *fakeRepo) GetSucceededByItineraryQuoteID(context.Context, int64) (*models.Payment, error) { return nil, models.ErrNotFound }

// fakeEnqueuer records finalize jobs so the test asserts at-most-once enqueue.
type fakeEnqueuer struct {
	mu    sync.Mutex
	calls []int64
}

func (e *fakeEnqueuer) EnqueuePaymentFinalize(_ context.Context, orderID int64, _ bool, _, _ string) error {
	e.mu.Lock(); defer e.mu.Unlock()
	e.calls = append(e.calls, orderID)
	return nil
}

func (e *fakeEnqueuer) EnqueueItineraryDepositFinalize(_ context.Context, quoteID int64, _ bool, _, _ string) error {
	e.mu.Lock(); defer e.mu.Unlock()
	e.calls = append(e.calls, quoteID)
	return nil
}

// mockRegistry builds a registry with the real MockGateway under "airwallex".
func mockRegistry(name string) *Registry {
	r := NewRegistry()
	mock := payments.NewMockGateway()
	r.Register(name, mock)
	r.Register("mock", mock)
	return r
}

// TestHandleWebhookIdempotent asserts a replayed webhook is a no-op:
// UpsertWebhook returns inserted=false and the finalize job enqueues once.
func TestHandleWebhookIdempotent(t *testing.T) {
	repo := newFakeRepo()
	enq := &fakeEnqueuer{}
	svc := NewService(repo, mockRegistry("airwallex"), nil, nil, enq, "https://example/return")

	body, _ := json.Marshal(map[string]any{
		"order_id": 42, "gateway_ref": "int_42", "status": "succeeded",
		"event_id": "evt_1", "amount_minor": 53550, "currency": "USD",
	})
	require.NoError(t, svc.HandleWebhook(context.Background(), "airwallex", body, nil))
	require.NoError(t, svc.HandleWebhook(context.Background(), "airwallex", body, nil)) // replay

	assert.Equal(t, []int64{42}, enq.calls, "finalize enqueued exactly once (idempotent)")
	assert.Len(t, repo.events, 1, "one event row — replay was a no-op")
}

// TestHandleWebhookRejectsInvalidSignature asserts an unverified webhook is
// rejected at the security boundary.
func TestHandleWebhookRejectsInvalidSignature(t *testing.T) {
	svc := NewService(newFakeRepo(), mockRegistry("airwallex"), nil, nil, &fakeEnqueuer{}, "")
	// Missing gateway_ref/event_id → MockGateway.VerifyWebhook → ErrWebhookSignatureInvalid.
	body, _ := json.Marshal(map[string]any{"order_id": 42, "status": "succeeded"})
	err := svc.HandleWebhook(context.Background(), "airwallex", body, nil)
	assert.ErrorIs(t, err, models.ErrWebhookSignatureInvalid)
}

// TestHandleWebhookUnknownGateway asserts an unknown gateway name is unavailable.
func TestHandleWebhookUnknownGateway(t *testing.T) {
	svc := NewService(newFakeRepo(), NewRegistry(), nil, nil, &fakeEnqueuer{}, "")
	err := svc.HandleWebhook(context.Background(), "stripe", []byte("{}"), nil)
	assert.ErrorIs(t, err, models.ErrGatewayUnavailable)
}

// TestRefundNoSucceededPayment asserts refund on an unpaid order returns
// ErrPaymentNotSucceeded (GetSucceededByOrderID → ErrNotFound).
func TestRefundNoSucceededPayment(t *testing.T) {
	svc := NewService(newFakeRepo(), mockRegistry("mock"), nil, nil, &fakeEnqueuer{}, "")
	err := svc.Refund(context.Background(), 42, "x")
	assert.ErrorIs(t, err, models.ErrPaymentNotSucceeded)
}
