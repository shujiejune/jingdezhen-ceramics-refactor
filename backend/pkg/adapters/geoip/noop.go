package geoip

// Noop is the dev/test Lookup: every IP resolves to ("ZZ", false) so the app
// runs without a downloaded GeoLite2 db. Selected by GEOIP_MODE=noop (default).
type Noop struct{}

// NewNoop returns a Noop lookup.
func NewNoop() *Noop { return &Noop{} }

// Country satisfies Lookup — always ("ZZ", false).
func (*Noop) Country(_ string) (string, bool) { return "ZZ", false }

// Close is a no-op; safe to call any number of times.
func (*Noop) Close() error { return nil }
