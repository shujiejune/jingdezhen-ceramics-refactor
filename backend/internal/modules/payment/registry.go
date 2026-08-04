package payment

import (
	"fmt"

	"jingdezhen-ceramics-backend/pkg/adapters/payments"
)

// Registry is a simple in-memory GatewayRegistry: name → Gateway. Built in
// main.go from the PAYMENTS_MODE config (mock → MockGateway registered under
// all three names; sandbox/live → real Airwallex + PayPal clients).
type Registry struct {
	byName map[string]payments.Gateway
}

func NewRegistry(gws ...payments.Gateway) *Registry {
	r := &Registry{byName: make(map[string]payments.Gateway, len(gws))}
	for _, g := range gws {
		r.byName[g.Name()] = g
	}
	return r
}

// Register adds a gateway under an explicit name (used in mock mode to map the
// single MockGateway under airwallex/paypal/mock so dev webhooks resolve).
func (r *Registry) Register(name string, g payments.Gateway) {
	r.byName[name] = g
}

// Get returns the gateway by name, or an error if none is registered (e.g.
// PAYMENTS_MODE=mock but the webhook names a real gateway — treated as
// unavailable). The order service maps the error to models.ErrGatewayUnavailable.
func (r *Registry) Get(name string) (payments.Gateway, error) {
	g, ok := r.byName[name]
	if !ok {
		return nil, fmt.Errorf("payments: gateway %q not registered", name)
	}
	return g, nil
}
