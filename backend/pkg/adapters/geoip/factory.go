package geoip

import "fmt"

// New selects a Lookup from a mode string: "noop" → Noop, "maxmind" → MaxMind
// (opening path). An unknown mode is treated as noop so a stale config value
// can never wedge the analytics ingest (the only consequence is country='ZZ').
func New(mode, path string) (Lookup, error) {
	switch mode {
	case "maxmind":
		mm, err := NewMaxMind(path)
		if err != nil {
			return nil, err
		}
		return mm, nil
	default: // "noop" or any unknown value
		return NewNoop(), nil
	}
}

// MustNew is New but panics on error — for startup wiring where a config error
// is a programmer error. Used in main; tests use New directly.
func MustNew(mode, path string) Lookup {
	l, err := New(mode, path)
	if err != nil {
		panic(fmt.Errorf("geoip.MustNew: %w", err))
	}
	return l
}
