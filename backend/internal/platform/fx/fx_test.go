package fx

import (
	"context"
	"testing"
	"time"

	"jingdezhen-ceramics-backend/internal/models"

	"github.com/shopspring/decimal"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// d is a test helper: parse a decimal from a string literal.
func d(s string) decimal.Decimal {
	v, err := decimal.NewFromString(s)
	if err != nil {
		panic(err)
	}
	return v
}

// =============================================================================
// RoundPrice — the PRD §3.2.3 rounding rule (priority test target, TDD §11).
//   - under 100: round UP to the nearest 0.50
//   - 100 and above: round UP to the nearest whole unit
// =============================================================================

func TestRoundPrice(t *testing.T) {
	tests := []struct {
		name string
		in   string
		want string
	}{
		// PRD example.
		{"PRD example 183.47 → 184.00", "183.47", "184"},

		// Under-100 band: ceil to 0.50.
		{"99.49 → 99.50", "99.49", "99.50"},
		{"99.99 → 100.00 (crosses band)", "99.99", "100"},
		{"50.50 unchanged", "50.50", "50.50"},
		{"50.01 → 50.50", "50.01", "50.50"},
		{"0.01 → 0.50 (floor of band)", "0.01", "0.50"},
		{"0.00 unchanged", "0", "0"},

		// 100+ band: ceil to whole unit.
		{"100.00 unchanged", "100", "100"},
		{"100.01 → 101.00", "100.01", "101"},
		{"250.00 unchanged", "250", "250"},
		{"250.01 → 251.00", "250.01", "251"},

		// 0.50-step boundary at 99.50: already aligned.
		{"99.50 unchanged", "99.50", "99.50"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := RoundPrice(d(tc.in))
			assert.True(t, got.Equal(d(tc.want)),
				"RoundPrice(%s) = %s, want %s", tc.in, got, tc.want)
		})
	}
}

// =============================================================================
// Convert — CNY minor units → presentment minor units, with rounding.
// =============================================================================

func TestConvert(t *testing.T) {
	// rate_to_cny(USD) = 7.0588 (i.e. 1 USD = 7.0588 CNY, post-markup).
	// ¥1280.00 = 128000 fen → 1280/7.0588 = 181.34 USD → ≥100 band → ceil to 182 → 18200 cents.
	rate := d("7.0588")
	got, err := Convert(128000, CurrencyUSD, rate)
	require.NoError(t, err)
	assert.Equal(t, int64(18200), got, "¥1280 at rate 7.0588 → $182.00")

	// Unsupported currency.
	_, err = Convert(128000, "JPY", rate)
	assert.Error(t, err)

	// Non-positive rate.
	_, err = Convert(128000, CurrencyUSD, decimal.Zero)
	assert.Error(t, err)
}

// =============================================================================
// Refresh — ECB EUR-base → derive CNY→{USD,EUR,GBP} → apply 2% markup → upsert.
// Verifies the markup direction (the classic bug: applying it backwards).
// =============================================================================

// fakeRepo is an in-memory RepositoryInterface for testing Refresh + Convert.
type fakeRepo struct {
	rates map[string]decimal.Decimal
}

func (f *fakeRepo) GetRate(_ context.Context, currency string) (decimal.Decimal, time.Time, error) {
	r, ok := f.rates[currency]
	if !ok {
		return decimal.Zero, time.Time{}, models.ErrRateNotFound
	}
	return r, time.Now(), nil
}

func (f *fakeRepo) UpsertRates(_ context.Context, rates map[string]decimal.Decimal, _ time.Time) error {
	if f.rates == nil {
		f.rates = map[string]decimal.Decimal{}
	}
	for k, v := range rates {
		f.rates[k] = v
	}
	return nil
}

func (f *fakeRepo) ListAll(_ context.Context) ([]RateRow, error) {
	out := []RateRow{}
	for k, v := range f.rates {
		out = append(out, RateRow{Currency: k, RateToCNY: v})
	}
	return out, nil
}

func TestRefreshMarkupDirection(t *testing.T) {
	// ECB EUR-base: 1 EUR = 1.0845 USD, 7.8521 CNY, 0.8530 GBP.
	src := FixtureRateSource{Rates: DefaultFixtureRates()}
	repo := &fakeRepo{}
	svc := NewService(repo, src, 200) // 200 bps = 2%

	require.NoError(t, svc.Refresh(context.Background()))

	// raw rate_to_cny(USD) = EUR→CNY / EUR→USD = 7.8521 / 1.0845 = 7.2372...
	// stored = raw / 1.02 = 7.0953...
	// The KEY assertion: stored rate is LESS than the raw rate (because dividing
	// by (1+markup) shrinks rate_to_cny, which makes presentmentMajor LARGER:
	//   presentment = cny / storedRate > cny / rawRate
	// i.e. the customer pays more USD per CNY — the merchant keeps the spread).
	rawUSD := d("7.8521").Div(d("1.0845"))
	storedUSD := repo.rates[CurrencyUSD]
	assert.True(t, storedUSD.LessThan(rawUSD),
		"markup must shrink rate_to_cny (got %s, raw %s) — customer pays more presentment",
		storedUSD, rawUSD)

	// End-to-end: a ¥1000 item converts to MORE presentment USD with the stored
	// (marked-up) rate than with the raw rate.
	withMarkup, _ := Convert(100000, CurrencyUSD, storedUSD)
	withoutMarkup, _ := Convert(100000, CurrencyUSD, rawUSD)
	assert.True(t, withMarkup > withoutMarkup,
		"with-markup presentment (%d) must exceed raw presentment (%d)",
		withMarkup, withoutMarkup)

	// Spot-check the exact stored value (7.8521 / 1.0845 / 1.02 = 7.09532...).
	expected := d("7.8521").Div(d("1.0845")).Div(d("1.02"))
	assert.True(t, storedUSD.Equal(expected),
		"stored USD rate = %s, want %s", storedUSD, expected)
}

// TestRefreshMissingCNY verifies the feed-guard: no CNY → refresh fails rather
// than upserting partial/garbage rates.
func TestRefreshMissingCNY(t *testing.T) {
	src := FixtureRateSource{Rates: map[string]decimal.Decimal{
		CurrencyUSD: d("1.0845"),
		CurrencyGBP: d("0.8530"),
		// CNY intentionally missing.
	}}
	repo := &fakeRepo{}
	svc := NewService(repo, src, 200)
	err := svc.Refresh(context.Background())
	require.Error(t, err)
	assert.Contains(t, err.Error(), "CNY")
}

// TestConvertAfterRefresh is the integration-flavored sanity check: Refresh
// then Convert returns a sane (non-zero) presentment amount.
func TestConvertAfterRefresh(t *testing.T) {
	src := FixtureRateSource{Rates: DefaultFixtureRates()}
	repo := &fakeRepo{}
	svc := NewService(repo, src, 200)

	require.NoError(t, svc.Refresh(context.Background()))
	got, err := svc.Convert(context.Background(), 128000, CurrencyUSD)
	require.NoError(t, err)
	assert.Greater(t, got, int64(0))
}
