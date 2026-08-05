package geoip

// NewTestMaxMind returns a MaxMind backed by the embedded GeoIP2-Country test
// fixture (81.2.69.160 → GB), for use by cross-package integration tests that
// need a real GeoIP lookup without depending on a file path or a MaxMind
// download. CWD-independent (the .mmdb is embedded into the binary).
//
// Tests outside this package must not call this in short mode; they should
// t.Skip() under testing.Short() themselves (the geoip reader is cheap but
// the test usually also needs a DB).
func NewTestMaxMind() (*MaxMind, error) {
	return NewMaxMindFromBytes(testMMDB)
}
