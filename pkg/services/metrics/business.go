package metrics

import (
	"go.opentelemetry.io/otel/metric"
)

// Provider is fixed for the process lifetime, so callers may cache instruments.
func (m *manager) Provider() metric.MeterProvider {
	return m.provider
}
