package geoip

import (
	"fmt"
	"net"
	"os"

	"github.com/oschwald/geoip2-golang"
)

// MaxMind reads a GeoLite2-Country (or GeoIP2-Country) .mmdb and resolves IPs
// to ISO 3166-1 alpha-2 codes. The underlying geoip2.Reader is safe for
// concurrent use. Selected by GEOIP_MODE=maxmind + GEOLITE2_DB_PATH.
type MaxMind struct {
	db *geoip2.Reader
}

// NewMaxMind opens the .mmdb at path. If path is empty or the file is missing
// it returns an error so a misconfigured live mode fails loudly at startup
// rather than silently degrading; callers that want a graceful fallback can
// catch the error and use NewNoop() instead.
func NewMaxMind(path string) (*MaxMind, error) {
	if path == "" {
		return nil, fmt.Errorf("geoip: GEOLITE2_DB_PATH is empty")
	}
	if _, err := os.Stat(path); err != nil {
		return nil, fmt.Errorf("geoip: stat %s: %w", path, err)
	}
	db, err := geoip2.Open(path)
	if err != nil {
		return nil, fmt.Errorf("geoip: open %s: %w", path, err)
	}
	return &MaxMind{db: db}, nil
}

// Country resolves ip to its ISO 3166-1 alpha-2 code. Private/unparseable
// addresses and lookup misses return ("ZZ", false).
func (m *MaxMind) Country(ip string) (string, bool) {
	addr := net.ParseIP(ip)
	if addr == nil {
		return "ZZ", false
	}
	c, err := m.db.Country(addr)
	if err != nil || c == nil || c.Country.IsoCode == "" {
		return "ZZ", false
	}
	return c.Country.IsoCode, true
}

// Close releases the .mmdb file handle.
func (m *MaxMind) Close() error {
	if m.db == nil {
		return nil
	}
	return m.db.Close()
}
