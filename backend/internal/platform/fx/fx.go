// Package fx implements the CNY→{USD,EUR,GBP} currency-conversion pipeline
// (TDD §7, §3.4; PRD §3.2.3).
//
// Pipeline: a daily job (fx:refresh, Asynq cron 16:05 CET) fetches ECB
// EUR-base reference rates, derives the CNY-per-currency rate, applies the
// operator-configurable markup (default 2%), and upserts into the fx_rates
// table. Read-time conversion (catalog, cart, checkout) calls Service.Convert,
// which reads the cached rate and applies the PRD rounding rule.
//
// Money is BIGINT minor units (fen) everywhere in Go; decimal arithmetic is
// confined to this package via shopspring/decimal (TDD §7: never float for
// money arithmetic). Rates are NUMERIC(18,8) in Postgres, scanned natively.
//
// Markup direction (easy to get backwards): rate_to_cny is stored as
// raw / (1 + markup), so the customer pays MORE presentment currency per CNY
// than the raw rate implies. A ¥1000 item at raw 7.0 CNY/USD = $142.86; with
// 2% markup stored rate = 6.8627, so presentment = $145.68. The merchant
// keeps the difference as FX-volatility cover.
package fx

import (
	"context"
	"fmt"
	"jingdezhen-ceramics-backend/internal/models"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/shopspring/decimal"
)

// Presentment currencies (PRD §3.2.3). CNY is base/settlement only — never a
// presentment choice — so it is excluded.
const (
	CurrencyUSD = "USD"
	CurrencyEUR = "EUR"
	CurrencyGBP = "GBP"
	CurrencyCNY = "CNY" // base/settlement only
)

// SupportedPresentment is the set of currencies a customer may be charged in.
var SupportedPresentment = []string{CurrencyUSD, CurrencyEUR, CurrencyGBP}

// IsSupportedPresentment reports whether code is a valid presentment currency.
func IsSupportedPresentment(code string) bool {
	for _, c := range SupportedPresentment {
		if c == code {
			return true
		}
	}
	return false
}

// --- Errors ------------------------------------------------------------------

// (ErrRateNotFound lives in internal/models/errors.go with the other domain
// errors, so the error-mapper middleware can centralize HTTP status mapping.)

// --- RateSource (TDD §4.1 adapter) -------------------------------------------
// ECB is the only live source for MVP; the interface exists so tests + dev can
// inject fixtures without hitting the network (TDD §4.1: "fixture rates in dev").

// RateSource returns EUR-base reference rates: a map of currency code → the
// ECB EUR exchange rate (1 EUR = rate units of currency). CNY is always present.
type RateSource interface {
	FetchRates(ctx context.Context) (map[string]decimal.Decimal, error)
}

// --- Pure conversion logic (unit-tested) -------------------------------------

// RoundPrice applies the PRD §3.2.3 rounding rule to a major-unit amount:
//   - under 100: round UP to the nearest 0.50
//   - 100 and above: round UP to the nearest whole unit
//
// Examples (PRD §3.2.3): 183.47 → 184.00; 99.49 → 99.50; 100.01 → 101.00.
func RoundPrice(major decimal.Decimal) decimal.Decimal {
	hundred := decimal.NewFromInt(100)
	if major.LessThan(hundred) {
		// Round up to the nearest 0.50.
		half := decimal.NewFromFloat(0.5)
		// ceil(major / 0.5) * 0.5
		n := major.Div(half).Ceil()
		return n.Mul(half)
	}
	// 100 and above: round up to the nearest whole unit.
	return major.Ceil()
}

// Convert converts a CNY minor-unit (fen) amount to a presentment-currency
// minor-unit amount, given the stored rate_to_cny (CNY per 1 unit of currency,
// already markup-adjusted). Applies the PRD rounding rule.
//
// presentmentMajor = (cnyMinor / 100) / rateToCNY
// presentmentMinor = RoundPrice(presentmentMajor) * 100
func Convert(cnyMinor int64, currency string, rateToCNY decimal.Decimal) (int64, error) {
	if !IsSupportedPresentment(currency) {
		return 0, fmt.Errorf("fx: unsupported presentment currency %q", currency)
	}
	if rateToCNY.Sign() <= 0 {
		return 0, fmt.Errorf("fx: non-positive rate_to_cny for %s", currency)
	}
	cnyMajor := decimal.NewFromInt(cnyMinor).Div(decimal.NewFromInt(100))
	presentmentMajor := cnyMajor.Div(rateToCNY)
	rounded := RoundPrice(presentmentMajor)
	return rounded.Mul(decimal.NewFromInt(100)).IntPart(), nil
}

// --- Repository (fx_rates table) ---------------------------------------------

// RepositoryInterface defines fx_rates storage operations.
type RepositoryInterface interface {
	// GetRate returns the stored rate_to_cny and its fetch time for a currency.
	GetRate(ctx context.Context, currency string) (decimal.Decimal, time.Time, error)
	// UpsertRates writes the given markup-adjusted rates (currency → rate_to_cny)
	// in a single transaction, touching fetched_at. Idempotent.
	UpsertRates(ctx context.Context, rates map[string]decimal.Decimal, fetchedAt time.Time) error
	// ListAll returns every fx_rates row (for the GET /fx/rates debug endpoint).
	ListAll(ctx context.Context) ([]RateRow, error)
}

// RateRow is one fx_rates row.
type RateRow struct {
	Currency   string          `json:"currency"`
	RateToCNY  decimal.Decimal `json:"rate_to_cny"`
	FetchedAt  time.Time       `json:"fetched_at"`
}

type Repository struct {
	db *pgxpool.Pool
}

func NewRepository(db *pgxpool.Pool) RepositoryInterface {
	return &Repository{db: db}
}

func (r *Repository) GetRate(ctx context.Context, currency string) (decimal.Decimal, time.Time, error) {
	var rate decimal.Decimal
	var fetchedAt time.Time
	err := r.db.QueryRow(ctx,
		`SELECT rate_to_cny, fetched_at FROM fx_rates WHERE currency = $1`, currency).
		Scan(&rate, &fetchedAt)
	if err != nil {
		if err.Error() == "no rows in result set" {
			return decimal.Zero, time.Time{}, models.ErrRateNotFound
		}
		return decimal.Zero, time.Time{}, fmt.Errorf("fx.Repository.GetRate: %w", err)
	}
	return rate, fetchedAt, nil
}

func (r *Repository) UpsertRates(ctx context.Context, rates map[string]decimal.Decimal, fetchedAt time.Time) error {
	tx, err := r.db.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return fmt.Errorf("fx.Repository.UpsertRates.BeginTx: %w", err)
	}
	defer tx.Rollback(ctx)

	for currency, rate := range rates {
		_, err := tx.Exec(ctx,
			`INSERT INTO fx_rates (currency, rate_to_cny, fetched_at)
			 VALUES ($1, $2, $3)
			 ON CONFLICT (currency) DO UPDATE SET rate_to_cny = EXCLUDED.rate_to_cny,
			                                       fetched_at = EXCLUDED.fetched_at`,
			currency, rate, fetchedAt)
		if err != nil {
			return fmt.Errorf("fx.Repository.UpsertRates(%s): %w", currency, err)
		}
	}
	return tx.Commit(ctx)
}

func (r *Repository) ListAll(ctx context.Context) ([]RateRow, error) {
	rows, err := r.db.Query(ctx,
		`SELECT currency, rate_to_cny, fetched_at FROM fx_rates ORDER BY currency`)
	if err != nil {
		return nil, fmt.Errorf("fx.Repository.ListAll: %w", err)
	}
	defer rows.Close()

	out := []RateRow{}
	for rows.Next() {
		var row RateRow
		if err := rows.Scan(&row.Currency, &row.RateToCNY, &row.FetchedAt); err != nil {
			return nil, fmt.Errorf("fx.Repository.ListAll.Scan: %w", err)
		}
		out = append(out, row)
	}
	return out, rows.Err()
}

// --- Service (refresh job + read-time convert) -------------------------------

// ServiceInterface defines the FX service surface used by the worker (Refresh)
// and by request handlers (Convert) + checkout (Rate, for the order snapshot).
type ServiceInterface interface {
	// Refresh fetches live rates, derives CNY→{USD,EUR,GBP}, applies the markup,
	// and upserts into fx_rates. Called by the fx:refresh worker job.
	Refresh(ctx context.Context) error
	// Convert returns the presentment-currency minor-unit amount for a CNY
	// minor-unit amount. Reads the stored (cached) rate.
	Convert(ctx context.Context, cnyMinor int64, currency string) (int64, error)
	// Rate returns the stored presentment→CNY rate for a currency, for the
	// order's fx_rate_used snapshot at checkout. Returns models.ErrRateNotFound
	// if the currency has no row (refresh not yet run).
	Rate(ctx context.Context, currency string) (decimal.Decimal, error)
}

// Service is the FX service. markup is applied at refresh time (stored in the
// rate), so Convert reads a single column and divides.
type Service struct {
	repo       RepositoryInterface
	source     RateSource
	markup     decimal.Decimal // e.g. 0.02 for 2%
}

// NewService builds the FX service. markupBPS is basis points (200 = 2%).
func NewService(repo RepositoryInterface, source RateSource, markupBPS int) ServiceInterface {
	return &Service{
		repo:   repo,
		source: source,
		markup: decimal.NewFromInt(int64(markupBPS)).Div(decimal.NewFromInt(10000)),
	}
}

// Refresh implements the fx:refresh job (TDD §4.2, §7).
func (s *Service) Refresh(ctx context.Context) error {
	eurBase, err := s.source.FetchRates(ctx)
	if err != nil {
		return fmt.Errorf("fx.Refresh.FetchRates: %w", err)
	}
	// ECB publishes EUR-base: 1 EUR = rate units of each currency. CNY is always
	// in the feed; if missing we cannot derive anything.
	eurToCNY, ok := eurBase[CurrencyCNY]
	if !ok || eurToCNY.Sign() <= 0 {
		return fmt.Errorf("fx.Refresh: ECB feed missing EUR→CNY rate")
	}
	// Derive rate_to_cny for each presentment currency and apply the markup.
	// raw rate_to_cny(X) = eurToCNY / eurToX  (CNY per 1 unit of X)
	// stored = raw / (1 + markup)  → customer pays more presentment per CNY
	// EUR is the ECB base currency, so it isn't in the feed — 1 EUR = 1 EUR is
	// implicit, and rate_to_cny(EUR) = eurToCNY directly.
	adjusted := make(map[string]decimal.Decimal, len(SupportedPresentment))
	divisor := decimal.NewFromInt(1).Add(s.markup)
	now := time.Now().UTC()
	for _, cur := range SupportedPresentment {
		var raw decimal.Decimal
		if cur == CurrencyEUR {
			raw = eurToCNY // 1 EUR = eurToCNY CNY
		} else {
			eurToX, ok := eurBase[cur]
			if !ok || eurToX.Sign() <= 0 {
				return fmt.Errorf("fx.Refresh: ECB feed missing EUR→%s rate", cur)
			}
			raw = eurToCNY.Div(eurToX) // CNY per 1 X
		}
		stored := raw.Div(divisor) // apply markup
		adjusted[cur] = stored
	}
	if err := s.repo.UpsertRates(ctx, adjusted, now); err != nil {
		return fmt.Errorf("fx.Refresh.Upsert: %w", err)
	}
	return nil
}

// Convert reads the stored rate for a currency and applies Convert + rounding.
func (s *Service) Convert(ctx context.Context, cnyMinor int64, currency string) (int64, error) {
	if !IsSupportedPresentment(currency) {
		return 0, fmt.Errorf("fx.Convert: unsupported currency %q", currency)
	}
	rate, _, err := s.repo.GetRate(ctx, currency)
	if err != nil {
		return 0, fmt.Errorf("fx.Convert: %w", err)
	}
	return Convert(cnyMinor, currency, rate)
}

// Rate returns the stored presentment→CNY rate for a currency (the snapshot value
// stored on orders at checkout). Returns models.ErrRateNotFound if absent.
func (s *Service) Rate(ctx context.Context, currency string) (decimal.Decimal, error) {
	if !IsSupportedPresentment(currency) {
		return decimal.Zero, fmt.Errorf("fx.Rate: unsupported currency %q", currency)
	}
	rate, _, err := s.repo.GetRate(ctx, currency)
	if err != nil {
		return decimal.Zero, models.ErrRateNotFound
	}
	return rate, nil
}

// --- helpers -----------------------------------------------------------------

// (Compile-time check that models is referenced for future currency/price DTOs.)
var _ = models.DefaultLocale
