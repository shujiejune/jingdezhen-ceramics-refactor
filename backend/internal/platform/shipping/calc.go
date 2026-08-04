// Package shipping holds the per-country weight-tiered shipping-fee calculator
// (PRD §3.2.3, TDD §7): fee = tier(country, ceil(Σ item.weight_grams * qty)).
//
// The calculator is pure (no DB, no FX) so it can be unit-tested in isolation
// and reused by checkout and the public /shipping/quote preview. The repository
// loads tiers; this package computes the fee.
package shipping

import (
	"sort"

	"jingdezhen-ceramics-backend/internal/models"
)

// Tier is one row of a country's weight-tiered fee table: orders weighing up to
// MaxWeightGrams (inclusive) cost FeeCNY minor units (fen).
type Tier struct {
	Country        string // ISO 3166-1 alpha-2
	MaxWeightGrams int
	FeeCNY         int64 // minor units (fen)
}

// CalcFee returns the CNY minor-unit shipping fee for an order of the given
// weight bound for the given country's tiers.
//
// Rules (TDD §7):
//   - No tiers for the country → ErrUnshippable (checkout blocked).
//   - Order weight exceeds the heaviest tier → ErrOverweight (checkout blocked;
//     handled manually per PRD §3.2.3 overweight message).
//   - Otherwise the fee of the lowest tier whose MaxWeightGrams >= weight.
//
// Tiers are sorted by MaxWeightGrams ascending so the first match is the
// cheapest sufficient tier (callers may pass unsorted rows).
func CalcFee(tiers []Tier, weightGrams int) (int64, error) {
	if len(tiers) == 0 {
		return 0, models.ErrUnshippable
	}
	if weightGrams < 0 {
		weightGrams = 0
	}
	sorted := make([]Tier, len(tiers))
	copy(sorted, tiers)
	sort.SliceStable(sorted, func(i, j int) bool {
		return sorted[i].MaxWeightGrams < sorted[j].MaxWeightGrams
	})
	heaviest := sorted[len(sorted)-1].MaxWeightGrams
	if weightGrams > heaviest {
		return 0, models.ErrOverweight
	}
	for _, t := range sorted {
		if weightGrams <= t.MaxWeightGrams {
			return t.FeeCNY, nil
		}
	}
	// Unreachable: weightGrams <= heaviest guarantees a match above.
	return 0, models.ErrOverweight
}

// TotalWeight sums each item's packed weight × qty (the ceil in TDD §7 is on
// the summed weight, not per-item — integer grams need no ceil).
func TotalWeight(unitWeightGrams []int, qty []int) int {
	total := 0
	for i := range unitWeightGrams {
		if i < len(qty) {
			total += unitWeightGrams[i] * qty[i]
		}
	}
	if total < 0 {
		return 0
	}
	return total
}
