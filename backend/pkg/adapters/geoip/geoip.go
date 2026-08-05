// Package geoip provides a MaxMind GeoLite2-backed country-lookup adapter for
// the in-house analytics ingest (TDD §3.4/§10, PRD §3.4.2).
//
// It follows the same env-flip convention as the other adapters
// (pkg/adapters/payments, /storage, /pdf): the Lookup interface is the seam the
// analytics service depends on, and the dev default (Noop) needs no external
// artefacts. Live mode reads a local GeoLite2-Country .mmdb.
//
// Unknown / private / unparseable IPs resolve to ("ZZ", false) — the ISO 3166
// user-assigned "unknown country" code (TDD §10/§11).
package geoip

import _ "embed"

//go:embed testdata/GeoIP2-Country-Test.mmdb
var testMMDB []byte

// Lookup resolves a client IP to an ISO 3166-1 alpha-2 country code. `ok` is
// false when the IP is unknown, private, or the adapter is the no-op (dev).
// Implementations must be safe for concurrent use.
type Lookup interface {
	// Country returns the 2-letter country code (e.g. "US","CN","GB") and
	// whether the lookup found a real entry. Always returns ("ZZ", false)
	// on miss/no-op, never an empty string.
	Country(ip string) (code string, ok bool)
	// Close releases any underlying reader resources. Noop panics if called.
	Close() error
}
