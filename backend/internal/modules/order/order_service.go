package order

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"jingdezhen-ceramics-backend/internal/models"
	platformshipping "jingdezhen-ceramics-backend/internal/platform/shipping"
	"strconv"

	"github.com/shopspring/decimal"
)

// --- Injected dependencies (narrow interfaces, satisfied by existing services) ---

// CartFetcher loads the signed-in user's cart (locale-aware, enriched).
type CartFetcher interface {
	GetCart(ctx context.Context, userID string, locale string) (*models.Cart, error)
}

// CartClearer empties a user's cart after a successful checkout.
type CartClearer interface {
	BulkRemove(ctx context.Context, userID string, skuIDs []int64) (int, error)
}

// AddressFetcher loads a saved address (must belong to the user).
type AddressFetcher interface {
	GetAddress(ctx context.Context, userID string, id int64) (*models.UserAddress, error)
}

// ShippingCalcer loads a country's tiers (the pure CalcFee runs in the service).
type ShippingCalcer interface {
	TiersForCountry(ctx context.Context, country string) ([]platformshipping.Tier, error)
}

// CheckoutFX converts CNY→presentment and exposes the raw rate for the snapshot.
type CheckoutFX interface {
	Convert(ctx context.Context, cnyMinor int64, currency string) (int64, error)
	// Rate returns the stored CNY rate for the currency (presentment per CNY).
	Rate(ctx context.Context, currency string) (decimal.Decimal, error)
}

// EmailEnqueuer enqueues a transactional email (order confirmations, etc.).
type EmailEnqueuer interface {
	EnqueueEmailSend(ctx context.Context, to, subject, plainText, html string) error
}

// PaymentEnqueuer enqueues a payment-finalize job (mock seam in dev).
type PaymentEnqueuer interface {
	EnqueuePaymentFinalize(ctx context.Context, orderID int64, success bool, gateway, gatewayRef string) error
}

// UserPrefFetcher loads the user's preferred presentment currency (default USD).
type UserPrefFetcher interface {
	PreferredCurrency(ctx context.Context, userID string) (string, error)
}

// PaymentIntenter creates a gateway payment intent for an order + records the
// pending payment row. Returns the hosted checkout URL. Implemented by
// payment.Service (kept as an interface to avoid an import cycle: payment
// imports order for MarkPaid/GetAdmin, so order imports payment only via this
// narrow interface).
type PaymentIntenter interface {
	CreateIntent(ctx context.Context, gatewayName string, orderID int64, amountMinor int64, currency string) (string, error)
}

// PaymentRefunder issues a full refund for an order's succeeded payment via
// the gateway, marks the payment refunded. Implemented by payment.Service.
type PaymentRefunder interface {
	Refund(ctx context.Context, orderID int64, reason string) error
}

// UserFetcher loads the user profile (for the customer email on the order
// confirmation). Implemented by user.Service.GetUserProfile.
type UserFetcher interface {
	GetUserProfile(ctx context.Context, userID string) (*models.User, error)
}

// ServiceInterface defines order business logic.
type ServiceInterface interface {
	Checkout(ctx context.Context, userID string, req models.CheckoutRequest, locale string) (*models.Order, error)
	MarkPaid(ctx context.Context, orderID int64) error
	Ship(ctx context.Context, orderID int64, req models.ShipOrderRequest) error
	Complete(ctx context.Context, orderID int64) error
	Cancel(ctx context.Context, userID string, orderID int64, req models.CancelOrderRequest) error
	Refund(ctx context.Context, orderID int64, req models.RefundOrderRequest) error
	ListMine(ctx context.Context, userID string, page, limit int) ([]models.Order, int, error)
	GetMine(ctx context.Context, userID string, orderID int64) (*models.Order, error)
	ListAdmin(ctx context.Context, status string, page, limit int) ([]models.Order, int, error)
	GetAdmin(ctx context.Context, orderID int64) (*models.Order, error)
}

type Service struct {
	repo        RepositoryInterface
	cart        CartFetcher
	cartClear   CartClearer
	address     AddressFetcher
	shipping    ShippingCalcer
	fx          CheckoutFX
	email       EmailEnqueuer
	payment     PaymentEnqueuer
	userPref    UserPrefFetcher
	paymentIntenter PaymentIntenter  // nil in mock mode (auto-finalize seam)
	paymentRefunder PaymentRefunder // nil if refunds go through a different path
	userFetcher    UserFetcher      // customer email for the order confirmation
	paymentsMode string // "mock" (dev) | "live" (#6, not yet configured)
}

func NewService(
	repo RepositoryInterface,
	cart CartFetcher,
	cartClear CartClearer,
	address AddressFetcher,
	shipping ShippingCalcer,
	fx CheckoutFX,
	email EmailEnqueuer,
	payment PaymentEnqueuer,
	userPref UserPrefFetcher,
	paymentIntenter PaymentIntenter,
	paymentRefunder PaymentRefunder,
	userFetcher UserFetcher,
	paymentsMode string,
) *Service {
	return &Service{
		repo: repo, cart: cart, cartClear: cartClear, address: address,
		shipping: shipping, fx: fx, email: email, payment: payment,
		userPref: userPref, paymentIntenter: paymentIntenter,
		paymentRefunder: paymentRefunder, userFetcher: userFetcher,
		paymentsMode: paymentsMode,
	}
}

// SetPaymentIntenter wires the gateway-intent client post-construction to
// break the order↔payment import cycle (payment.Service needs order.Service as
// OrderFinalizer/OrderLoader; order.Service needs payment.Service as
// PaymentIntenter). Called in main.go after both services are built.
func (s *Service) SetPaymentIntenter(pi PaymentIntenter) { s.paymentIntenter = pi }

// SetPaymentRefunder wires the gateway-refund client post-construction.
func (s *Service) SetPaymentRefunder(pr PaymentRefunder) { s.paymentRefunder = pr }

// Checkout creates an order from the user's cart (PRD §3.2.3, TDD §7/§8):
//  1. Load + validate cart (non-empty).
//  2. Load the saved address (must belong to the user) → snapshot as JSONB.
//  3. Compute total weight → shipping fee (tier/overweight/unshippable).
//  4. Convert each unit price + shipping fee to presentment (reconciled:
//     line = unit × qty, subtotal = Σ line, total = subtotal + shipping).
//  5. Snapshot fx_rate_used; compute CNY totals for settlement.
//  6. CreateOrder (atomic stock decrement in tx; ErrConflict if insufficient).
//  7. Clear the cart; enqueue order-confirmation email.
//  8. Mock payments: enqueue payment:finalize{success} to drive created→paid.
func (s *Service) Checkout(ctx context.Context, userID string, req models.CheckoutRequest, locale string) (*models.Order, error) {
	// 1. Cart (non-empty).
	cart, err := s.cart.GetCart(ctx, userID, locale)
	if err != nil {
		return nil, fmt.Errorf("order.Checkout.GetCart: %w", err)
	}
	if len(cart.Items) == 0 {
		return nil, models.ErrCartEmpty
	}

	// Currency: explicit > user pref > USD.
	currency := req.Currency
	if currency == "" {
		currency, err = s.userPref.PreferredCurrency(ctx, userID)
		if err != nil || currency == "" {
			currency = "USD"
		}
	}
	if !isSupportedCurrency(currency) {
		return nil, models.ErrInvalidOperation
	}

	// 2. Address (must belong to the user).
	addr, err := s.address.GetAddress(ctx, userID, req.AddressID)
	if err != nil {
		if errors.Is(err, models.ErrNotFound) {
			return nil, models.ErrNotFound
		}
		return nil, fmt.Errorf("order.Checkout.Address: %w", err)
	}
	addrJSON, err := json.Marshal(addr)
	if err != nil {
		return nil, fmt.Errorf("order.Checkout.AddrMarshal: %w", err)
	}

	// 3. Shipping fee (CNY).
	weight := 0
	for _, it := range cart.Items {
		weight += it.WeightGrams * it.Qty
	}
	tiers, err := s.shipping.TiersForCountry(ctx, addr.Country)
	if err != nil {
		return nil, fmt.Errorf("order.Checkout.ShippingTiers: %w", err)
	}
	shippingCNY, err := platformshipping.CalcFee(tiers, weight)
	if err != nil {
		return nil, err // ErrUnshippable | ErrOverweight
	}

	// 4. Presentment (reconciled): line = unit × qty, subtotal = Σ line.
	items := make([]models.OrderItem, len(cart.Items))
	var subtotalMinor, subtotalCNY int64
	for i, ci := range cart.Items {
		unitMinor, err := s.fx.Convert(ctx, ci.UnitPriceCNY, currency)
		if err != nil {
			return nil, fmt.Errorf("order.Checkout.Convert(sku=%d): %w", ci.SkuID, err)
		}
		lineMinor := unitMinor * int64(ci.Qty)
		titleSnap, _ := json.Marshal(map[string]string{"title": ci.ProductTitle, "slug": ci.ProductSlug})
		items[i] = models.OrderItem{
			SkuID:             ci.SkuID,
			Qty:               ci.Qty,
			UnitPriceMinor:    unitMinor,
			UnitPriceCNY:      ci.UnitPriceCNY,
			TitleSnapshot:     titleSnap,
			AttributesSnapshot: ci.Attributes,
		}
		subtotalMinor += lineMinor
		subtotalCNY += ci.LineTotalCNY
	}
	shippingMinor, err := s.fx.Convert(ctx, shippingCNY, currency)
	if err != nil {
		return nil, fmt.Errorf("order.Checkout.ConvertShipping: %w", err)
	}
	totalMinor := subtotalMinor + shippingMinor
	totalCNY := subtotalCNY + shippingCNY

	// 5. fx_rate_used snapshot.
	rate, err := s.fx.Rate(ctx, currency)
	if err != nil {
		return nil, fmt.Errorf("order.Checkout.Rate: %w", err)
	}
	rateStr := rate.StringFixed(8)

	loc := locale
	o := &models.Order{
		UserID: userID, Currency: currency,
		SubtotalMinor: subtotalMinor, ShippingMinor: shippingMinor, TotalMinor: totalMinor,
		SubtotalCNY: subtotalCNY, ShippingCNY: shippingCNY, TotalCNY: totalCNY,
		FxRateUsedRaw: &rateStr, Address: addrJSON, Locale: &loc,
	}

	// 6. Create order (atomic stock decrement).
	orderID, err := s.repo.CreateOrder(ctx, o, items)
	if err != nil {
		return nil, err // ErrConflict (insufficient stock) wrapped
	}
	o.ID = orderID
	o.Status = models.StatusCreated

	// 7. Clear the cart.
	skuIDs := make([]int64, len(cart.Items))
	for i, ci := range cart.Items {
		skuIDs[i] = ci.SkuID
	}
	if _, err := s.cartClear.BulkRemove(ctx, userID, skuIDs); err != nil {
		log.Printf("order.Checkout.ClearCart(order=%d): %v (order still placed)", orderID, err)
	}

	// 8. Payment. mock mode: auto-succeed (dev seam). sandbox/live: create a
	// gateway intent + return the hosted checkout URL so the client redirects.
	if s.paymentsMode == "mock" {
		if err := s.payment.EnqueuePaymentFinalize(ctx, orderID, true, "mock", "mock-"+strconv.FormatInt(orderID, 10)); err != nil {
			log.Printf("order.Checkout.EnqueuePayment(order=%d): %v (order placed, manual finalize needed)", orderID, err)
		}
	} else if s.paymentIntenter != nil {
		hosted, err := s.paymentIntenter.CreateIntent(ctx, req.Gateway, orderID, totalMinor, currency)
		if err != nil {
			log.Printf("order.Checkout.CreateIntent(order=%d): %v (order left in `created`; customer can cancel)", orderID, err)
			return nil, fmt.Errorf("%w: %v", models.ErrGatewayUnavailable, err)
		}
		o.HostedURL = hosted // surface to the client for redirect
	}

	// Enqueue order-confirmation email (best-effort) to the real customer email.
	emailTo := ""
	if s.userFetcher != nil {
		if u, err := s.userFetcher.GetUserProfile(ctx, userID); err == nil {
			emailTo = u.Email
		}
	}
	if emailTo == "" {
		emailTo = "customer@example.com"
	}
	if err := s.email.EnqueueEmailSend(ctx, emailTo, "Order confirmed", "Your order #"+strconv.FormatInt(orderID, 10)+" was received.", ""); err != nil {
		log.Printf("order.Checkout.Email(order=%d): %v", orderID, err)
	}

	return s.repo.GetByID(ctx, orderID)
}

// MarkPaid moves created→paid (called by the worker's payment:finalize handler
// on a successful payment). Idempotent: a replayed webhook that arrives after
// the order is already paid returns nil (the gateway may retry; the
// idempotency_key on the payments row + this idempotent transition together
// guarantee at-most-once side effects, TDD §11).
func (s *Service) MarkPaid(ctx context.Context, orderID int64) error {
	if err := s.repo.TransitionStatus(ctx, orderID, models.StatusCreated, models.StatusPaid, ""); err != nil {
		if errors.Is(err, models.ErrConflict) {
			// Already paid (replayed webhook) — idempotent success.
			return nil
		}
		return err
	}
	return nil
}

func (s *Service) Ship(ctx context.Context, orderID int64, req models.ShipOrderRequest) error {
	return s.repo.SetShipped(ctx, orderID, req.CarrierName, req.TrackingNumber)
}

func (s *Service) Complete(ctx context.Context, orderID int64) error {
	return s.repo.TransitionStatus(ctx, orderID, models.StatusShipped, models.StatusCompleted, "")
}

// Cancel moves created→cancelled and restores stock. Customer may cancel before
// shipment (PRD §3.2.3 refund rule 1: before shipment, free cancellation).
func (s *Service) Cancel(ctx context.Context, userID string, orderID int64, req models.CancelOrderRequest) error {
	o, err := s.repo.GetByIDForUser(ctx, userID, orderID)
	if err != nil {
		return err
	}
	return s.repo.SetCancelled(ctx, orderID, req.Reason, o.Items)
}

// Refund moves paid|shipped→refunded (operator, full refunds only — PRD §3.2.3).
// Fail-closed: the gateway refund is called first; a gateway error leaves the
// order paid (no status transition). The payment row is marked refunded by the
// payment service after the gateway confirms.
func (s *Service) Refund(ctx context.Context, orderID int64, req models.RefundOrderRequest) error {
	o, err := s.repo.GetByID(ctx, orderID)
	if err != nil {
		return err
	}
	if o.Status != models.StatusPaid && o.Status != models.StatusShipped {
		return models.ErrConflict
	}
	// Issue the gateway refund first (fail-closed).
	if s.paymentRefunder != nil {
		if err := s.paymentRefunder.Refund(ctx, orderID, req.Reason); err != nil {
			return err
		}
	}
	return s.repo.TransitionStatus(ctx, orderID, o.Status, models.StatusRefunded, "")
}

func (s *Service) ListMine(ctx context.Context, userID string, page, limit int) ([]models.Order, int, error) {
	return s.repo.ListForUser(ctx, userID, page, limit)
}

func (s *Service) GetMine(ctx context.Context, userID string, orderID int64) (*models.Order, error) {
	return s.repo.GetByIDForUser(ctx, userID, orderID)
}

func (s *Service) ListAdmin(ctx context.Context, status string, page, limit int) ([]models.Order, int, error) {
	return s.repo.ListAdmin(ctx, status, page, limit)
}

func (s *Service) GetAdmin(ctx context.Context, orderID int64) (*models.Order, error) {
	return s.repo.GetByID(ctx, orderID)
}

// isSupportedCurrency checks the presentment-currency set (mirrors the cart +
// product handlers' local check; the set is fixed for MVP per PRD §3.2.3).
func isSupportedCurrency(code string) bool {
	switch code {
	case "USD", "EUR", "GBP":
		return true
	}
	return false
}
