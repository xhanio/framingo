package tracing

import (
	"go.opentelemetry.io/otel/propagation"
	"go.opentelemetry.io/otel/trace"
)

// Provider returns the active TracerProvider. Callers resolve it per use
// rather than caching the result: Init swaps in a fresh provider on every
// restart, and a retired provider silently drops spans.
func (m *manager) Provider() trace.TracerProvider {
	return m.controller.current()
}

func (m *manager) Propagator() propagation.TextMapPropagator {
	return m.propagator
}
