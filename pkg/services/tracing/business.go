package tracing

import (
	"go.opentelemetry.io/otel/propagation"
	"go.opentelemetry.io/otel/trace"
)

// Provider is the same for the manager's life, and may be kept, as may a
// Tracer from it: each span goes to the pipeline Init last built.
func (m *manager) Provider() trace.TracerProvider {
	return m.provider
}

func (m *manager) Propagator() propagation.TextMapPropagator {
	return m.propagator
}
