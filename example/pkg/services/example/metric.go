package example

import (
	otelmetric "go.opentelemetry.io/otel/metric"

	"github.com/xhanio/errors"
)

// metric groups the service's instruments, built once at Init.
type metric struct {
	hello otelmetric.Int64Counter
}

// newMetric creates the instruments; an invalid instrument fails Init at
// first boot instead of degrading silently.
func (m *manager) newMetric() (*metric, error) {
	hello, err := m.meter.Provider().Meter(m.name).Int64Counter("example.hello.total",
		otelmetric.WithDescription("HelloWorld requests handled"))
	if err != nil {
		return nil, errors.Wrap(err)
	}
	return &metric{hello: hello}, nil
}
