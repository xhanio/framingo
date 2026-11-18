package tracing

import (
	"go.opentelemetry.io/otel/propagation"
	"go.opentelemetry.io/otel/trace"
)

// Provider must be resolved per use, not cached: a restart retires it and it
// then silently drops spans.
func (m *manager) Provider() trace.TracerProvider {
	return m.controller.current()
}

func (m *manager) Propagator() propagation.TextMapPropagator {
	return m.propagator
}
