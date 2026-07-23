package cart

import (
	"context"
	"errors"
	"math"
	"testing"

	"jingdezhen-ceramics-backend/internal/models"
)

// mockConv is a stand-in for fx.Service.Convert: converts CNY minor units (fen)
// to presentment minor units (cents) at a fixed rate, applying the PRD §3.2.3
// ≥100 rounding band (ceil to whole units). Uses float only because the mock
// controls its inputs to avoid ambiguity; the real fx.Convert uses decimal.
type mockConv struct{ rate float64 }

func (m mockConv) Convert(_ context.Context, cnyMinor int64, _ string) (int64, error) {
	major := float64(cnyMinor) / 100.0 / m.rate
	return int64(math.Ceil(major)) * 100, nil
}

// errConv always fails — simulates a missing fx_rates row.
type errConv struct{}

func (errConv) Convert(_ context.Context, _ int64, _ string) (int64, error) {
	return 0, errors.New("fx: rate not found for currency")
}

// TestApplyPresentmentReconciliation asserts the cart fully reconciles in
// presentment currency: unit × qty == line, and Σ line == total. The total is
// the sum of rounded lines, NOT an independent rounding of the CNY lump sum
// (which would disagree with the displayed per-line amounts).
func TestApplyPresentmentReconciliation(t *testing.T) {
	// rate 7.1: ¥1280 → $181 (ceil 180.28); ¥880 → $124 (ceil 123.94).
	conv := mockConv{rate: 7.1}
	cart := &models.Cart{
		Items: []models.CartItem{
			{SkuID: 1, Qty: 5, UnitPriceCNY: 128000, LineTotalCNY: 640000}, // ¥1280 ×5
			{SkuID: 2, Qty: 1, UnitPriceCNY: 88000, LineTotalCNY: 88000},   // ¥880 ×1
		},
		TotalCNY: 728000, // ¥7280
	}

	applyPresentmentConv(context.Background(), "USD", conv, cart)

	wantUnit1 := int64(18100) // $181.00
	wantLine1 := wantUnit1 * 5 // $905.00
	wantUnit2 := int64(12400)  // $124.00
	wantLine2 := wantUnit2 * 1 // $124.00
	wantTotal := wantLine1 + wantLine2 // $1029.00

	// Per-line: unit × qty == line.
	if got := cart.Items[0].UnitPrice; got == nil || *got != wantUnit1 {
		t.Fatalf("item0 unit = %v, want %d", got, wantUnit1)
	}
	if got := cart.Items[0].LineTotal; got == nil || *got != wantLine1 {
		t.Fatalf("item0 line = %v, want %d (unit×qty)", got, wantLine1)
	}
	if got := cart.Items[1].UnitPrice; got == nil || *got != wantUnit2 {
		t.Fatalf("item1 unit = %v, want %d", got, wantUnit2)
	}
	if got := cart.Items[1].LineTotal; got == nil || *got != wantLine2 {
		t.Fatalf("item1 line = %v, want %d (unit×qty)", got, wantLine2)
	}

	// Σ line == total.
	if got := cart.Total; got == nil || *got != wantTotal {
		t.Fatalf("total = %v, want %d (sum of lines)", got, wantTotal)
	}

	// And the total must NOT equal an independent lump conversion of TotalCNY
	// (¥7280 → $1026) — that would disagree with the displayed lines.
	lump := int64(math.Ceil(7280.0/7.1)) * 100 // $1026.00
	if *cart.Total == lump {
		t.Fatalf("total == lump conversion %d; expected sum-of-lines %d (reconciliation broken)", lump, wantTotal)
	}
}

// TestApplyPresentmentDegradation asserts that when conversion can't happen
// (unsupported currency, no converter, empty currency, or conversion error),
// the cart stays CNY-only — no partial presentment fields are set.
func TestApplyPresentmentDegradation(t *testing.T) {
	makeCart := func() *models.Cart {
		return &models.Cart{
			Items:    []models.CartItem{{SkuID: 1, Qty: 1, UnitPriceCNY: 128000, LineTotalCNY: 128000}},
			TotalCNY: 128000,
		}
	}
	cases := []struct {
		name string
		cur  string
		conv PriceConverter
	}{
		{"empty currency", "", mockConv{7.1}},
		{"unsupported currency", "JPY", mockConv{7.1}},
		{"no converter", "USD", nil},
		{"conversion error", "USD", errConv{}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			c := makeCart()
			applyPresentmentConv(context.Background(), tc.cur, tc.conv, c)
			if c.Items[0].UnitPrice != nil || c.Items[0].LineTotal != nil || c.Total != nil || c.Currency != nil {
				t.Fatalf("%s: expected CNY-only cart, got presentment fields set", tc.name)
			}
		})
	}
}
