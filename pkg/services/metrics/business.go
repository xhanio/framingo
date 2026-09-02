package metrics

import (
	"go.opentelemetry.io/otel/metric"
)

// Provider returns the MeterProvider. Unlike tracing's, it is the same
// object for the life of the process - callers create instruments once and
// cache them; a restart swaps the exporter behind the shared reader, never
// the provider itself.
func (m *manager) Provider() metric.MeterProvider {
	return m.provider
}
