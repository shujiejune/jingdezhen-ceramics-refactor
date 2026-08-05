package geoip

import (
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// testDB is the MaxMind-DB test-data GeoIP2-Country sample. 81.2.69.160 → GB.
const testDB = "testdata/GeoIP2-Country-Test.mmdb"

func TestNoop_AlwaysZZ(t *testing.T) {
	n := NewNoop()
	defer n.Close()
	code, ok := n.Country("81.2.69.160")
	assert.Equal(t, "ZZ", code)
	assert.False(t, ok)
	// empty / garbage also ZZ, never empty
	code, _ = n.Country("")
	assert.Equal(t, "ZZ", code)
}

func TestFactory_NoopDefault(t *testing.T) {
	l, err := New("noop", "")
	require.NoError(t, err)
	defer l.Close()
	code, ok := l.Country("81.2.69.160")
	assert.Equal(t, "ZZ", code)
	assert.False(t, ok)
}

func TestFactory_UnknownModeFallsBackToNoop(t *testing.T) {
	// A stale/misconfigured mode must not wedge ingest.
	l, err := New("banana", "")
	require.NoError(t, err)
	defer l.Close()
	_, ok := l.Country("1.2.3.4")
	assert.False(t, ok)
}

func TestFactory_MaxMindMissingPathErrors(t *testing.T) {
	_, err := New("maxmind", "/nonexistent/GeoLite2.mmdb")
	require.Error(t, err)
	_, err = New("maxmind", "")
	require.Error(t, err) // empty path is a config error in live mode
}

func TestMaxMind_KnownIPResolves(t *testing.T) {
	mm, err := NewMaxMind(filepath.FromSlash(testDB))
	require.NoError(t, err)
	defer mm.Close()

	// 81.2.69.160 → GB (MaxMind test-data fixture).
	code, ok := mm.Country("81.2.69.160")
	assert.True(t, ok)
	assert.Equal(t, "GB", code)
}

func TestMaxMind_PrivateAndUnknownAreZZ(t *testing.T) {
	mm, err := NewMaxMind(filepath.FromSlash(testDB))
	require.NoError(t, err)
	defer mm.Close()

	for _, ip := range []string{"127.0.0.1", "10.0.0.1", "192.168.1.1", "0.0.0.0"} {
		code, ok := mm.Country(ip)
		assert.Equalf(t, "ZZ", code, "private %s should be ZZ", ip)
		assert.Falsef(t, ok, "private %s should be miss", ip)
	}
}

func TestMaxMind_GarbageIPisZZ(t *testing.T) {
	mm, err := NewMaxMind(filepath.FromSlash(testDB))
	require.NoError(t, err)
	defer mm.Close()
	for _, bad := range []string{"", "not-an-ip", "999.999.999.999", "abc:::z"} {
		code, ok := mm.Country(bad)
		assert.Equal(t, "ZZ", code)
		assert.False(t, ok)
	}
}

func TestMaxMind_IPv6(t *testing.T) {
	mm, err := NewMaxMind(filepath.FromSlash(testDB))
	require.NoError(t, err)
	defer mm.Close()
	// Unmapped IPv6 → ZZ (not in the country test db), but must not panic.
	code, ok := mm.Country("2001:4860:4860::8888")
	assert.False(t, ok)
	assert.Equal(t, "ZZ", code)
}

func TestMustNew_PanicsOnBadLiveConfig(t *testing.T) {
	assert.Panics(t, func() { MustNew("maxmind", "/nonexistent") })
}
