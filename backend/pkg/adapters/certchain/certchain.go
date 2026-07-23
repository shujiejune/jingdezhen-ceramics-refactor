// Package certchain is the reserved blockchain authentication-platform adapter
// (PRD §5.4). The certificate service calls it at certificate issue + sale so
// the authenticity chain can be registered on-chain when a real vendor is
// selected; for v1 the NoopChain does nothing (PRD: "vendor selected only when
// a real business need appears"). Built behind an interface so swapping in a
// real adapter is an env-var flip (TDD §4.1).
package certchain

import (
	"context"
	"encoding/json"
)

// Chain is the contract a blockchain authenticity provider satisfies. Methods
// register a certificate lifecycle event on-chain + return a transaction
// reference (stored in the provenance detail for audit). The NoopChain returns
// empty strings — no on-chain registration in v1.
type Chain interface {
	// RegisterCreation registers a certificate's creation (issue).
	RegisterCreation(ctx context.Context, certCode string, productID int64, detail json.RawMessage) (txRef string, err error)
	// RegisterSale registers a sale event on an existing certificate.
	RegisterSale(ctx context.Context, certCode string, orderID int64, detail json.RawMessage) (txRef string, err error)
}

// NoopChain is the v1 implementation — does nothing (PRD §5.4 reserved).
type NoopChain struct{}

func NewNoopChain() *NoopChain { return &NoopChain{} }

func (NoopChain) RegisterCreation(context.Context, string, int64, json.RawMessage) (string, error) {
	return "", nil
}

func (NoopChain) RegisterSale(context.Context, string, int64, json.RawMessage) (string, error) {
	return "", nil
}
