package shipping

import (
	"errors"
	"jingdezhen-ceramics-backend/internal/models"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestCalcFee(t *testing.T) {
	tiers := []Tier{
		{Country: "US", MaxWeightGrams: 1000, FeeCNY: 12000},
		{Country: "US", MaxWeightGrams: 3000, FeeCNY: 22000},
		{Country: "US", MaxWeightGrams: 5000, FeeCNY: 35000},
	}

	cases := []struct {
		name   string
		weight int
		want   int64
	}{
		{"zero weight → cheapest tier", 0, 12000},
		{"under first tier", 500, 12000},
		{"exact first-tier boundary (inclusive)", 1000, 12000},
		{"just over first tier → second", 1001, 22000},
		{"exact second-tier boundary", 3000, 22000},
		{"just over second tier → third", 3001, 35000},
		{"exact heaviest boundary", 5000, 35000},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, err := CalcFee(tiers, tc.weight)
			assert.NoError(t, err)
			assert.Equal(t, tc.want, got)
		})
	}

	// Unsorted input still picks the cheapest sufficient tier.
	unsorted := []Tier{
		{Country: "US", MaxWeightGrams: 5000, FeeCNY: 35000},
		{Country: "US", MaxWeightGrams: 1000, FeeCNY: 12000},
		{Country: "US", MaxWeightGrams: 3000, FeeCNY: 22000},
	}
	got, err := CalcFee(unsorted, 1001)
	assert.NoError(t, err)
	assert.Equal(t, int64(22000), got)
}

func TestCalcFeeOverweight(t *testing.T) {
	tiers := []Tier{{Country: "US", MaxWeightGrams: 5000, FeeCNY: 35000}}
	_, err := CalcFee(tiers, 5001)
	if !errors.Is(err, models.ErrOverweight) {
		t.Fatalf("got err=%v, want ErrOverweight", err)
	}
}

func TestCalcFeeUnshippable(t *testing.T) {
	_, err := CalcFee(nil, 100)
	if !errors.Is(err, models.ErrUnshippable) {
		t.Fatalf("got err=%v, want ErrUnshippable", err)
	}
	// A country with no tiers (empty slice) is also unshippable.
	_, err = CalcFee([]Tier{}, 100)
	if !errors.Is(err, models.ErrUnshippable) {
		t.Fatalf("got err=%v, want ErrUnshippable", err)
	}
}

func TestTotalWeight(t *testing.T) {
	got := TotalWeight([]int{1200, 900, 400}, []int{2, 1, 3}) // 2400+900+1200=4500
	if got != 4500 {
		t.Fatalf("TotalWeight = %d, want 4500", got)
	}
	if TotalWeight(nil, nil) != 0 {
		t.Fatalf("TotalWeight(nil) != 0")
	}
}
