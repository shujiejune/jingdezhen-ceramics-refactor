package fx

import (
	"context"
	"encoding/xml"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/shopspring/decimal"
)

// ECBClient fetches the ECB daily reference rates (EUR-base) from the official
// XML feed. TDD §4.1 adapter: "ECB FX | fx.RateSource | fixture rates in dev".
//
// Feed: https://www.ecb.europa.eu/stats/eurofxref/eurofxref-daily.xml
// Format (stable since ~2004): a gesmes:Envelope containing a Cube of Cubes:
//
//	<gesmes:Envelope>
//	  <Cube>
//	    <Cube time="2026-07-23">
//	      <Cube currency="USD" rate="1.0845"/>
//	      <Cube currency="CNY" rate="7.8521"/>
//	      ...
//	    </Cube>
//	  </Cube>
//	</gesmes:Envelope>
//
// Only the latest day's Cube is parsed (the feed is daily, not historical).
type ECBClient struct {
	endpoint string
	client   *http.Client
}

// NewECBClient builds a live ECB rate source. endpoint may be empty to use the
// default official feed.
func NewECBClient(endpoint string) *ECBClient {
	if endpoint == "" {
		endpoint = "https://www.ecb.europa.eu/stats/eurofxref/eurofxref-daily.xml"
	}
	return &ECBClient{
		endpoint: endpoint,
		client:   &http.Client{Timeout: 15 * time.Second},
	}
}

// FetchRates parses the ECB XML into a map of currency → rate (1 EUR = rate).
func (c *ECBClient) FetchRates(ctx context.Context) (map[string]decimal.Decimal, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.endpoint, nil)
	if err != nil {
		return nil, fmt.Errorf("fx.ECB.FetchRates.NewRequest: %w", err)
	}
	resp, err := c.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("fx.ECB.FetchRates.Do: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("fx.ECB.FetchRates: ECB returned HTTP %d", resp.StatusCode)
	}
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("fx.ECB.FetchRates.Read: %w", err)
	}
	return parseECBXML(body)
}

// --- ECB XML schema ----------------------------------------------------------
// We only declare the fields we read; xml.Unmarshal ignores the rest.

type ecEnvelope struct {
	Cube ecCube `xml:"Cube"`
}

type ecCube struct {
	Time []ecTimeCube `xml:"Cube"` // <Cube time="..."> entries
}

type ecTimeCube struct {
	Rates []ecRate `xml:"Cube"` // <Cube currency="USD" rate="1.0845"/>
}

type ecRate struct {
	Currency string `xml:"currency,attr"`
	Rate     string `xml:"rate,attr"`
}

func parseECBXML(body []byte) (map[string]decimal.Decimal, error) {
	var env ecEnvelope
	if err := xml.Unmarshal(body, &env); err != nil {
		return nil, fmt.Errorf("fx.ECB.parse: %w", err)
	}
	if len(env.Cube.Time) == 0 {
		return nil, fmt.Errorf("fx.ECB.parse: no time cube in feed")
	}
	// The latest day is the last <Cube time=...> entry.
	latest := env.Cube.Time[len(env.Cube.Time)-1]
	out := make(map[string]decimal.Decimal, len(latest.Rates))
	for _, r := range latest.Rates {
		rate, err := decimal.NewFromString(r.Rate)
		if err != nil {
			// Skip a malformed rate rather than fail the whole refresh; ECB
			// feed is well-formed in practice.
			continue
		}
		out[r.Currency] = rate
	}
	if _, ok := out[CurrencyCNY]; !ok {
		return nil, fmt.Errorf("fx.ECB.parse: CNY missing from ECB feed")
	}
	return out, nil
}

// --- FixtureRateSource (dev + tests) -----------------------------------------
// TDD §4.1: "fixture rates in dev". When ECB_API_URL is empty (dev), or in
// unit tests, inject this instead of ECBClient so no network call is made.

// FixtureRateSource returns a fixed map of EUR-base rates.
type FixtureRateSource struct {
	Rates map[string]decimal.Decimal
}

func (f FixtureRateSource) FetchRates(_ context.Context) (map[string]decimal.Decimal, error) {
	// Return a copy so callers can't mutate the fixture.
	out := make(map[string]decimal.Decimal, len(f.Rates))
	for k, v := range f.Rates {
		out[k] = v
	}
	return out, nil
}

// DefaultFixtureRates is a realistic EUR-base snapshot for dev: 1 EUR =
// ~1.0845 USD, ~7.8521 CNY, ~0.8530 GBP. (Approximate; dev-only.)
func DefaultFixtureRates() map[string]decimal.Decimal {
	return map[string]decimal.Decimal{
		CurrencyUSD: decimal.NewFromFloat(1.0845),
		CurrencyCNY: decimal.NewFromFloat(7.8521),
		CurrencyGBP: decimal.NewFromFloat(0.8530),
	}
}
