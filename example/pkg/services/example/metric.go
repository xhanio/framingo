package example

import (
	"go.opentelemetry.io/otel/metric"

	"github.com/xhanio/errors"
)

// metrics groups the service's instruments, built once at Init.
type metrics struct {
	hello metric.Int64Counter
}

// newMetrics creates the instruments; an invalid instrument fails Init at
// first boot instead of degrading silently.
func (m *manager) newMetrics() (*metrics, error) {
	hello, err := m.meter.Provider().Meter(m.name).Int64Counter("example.hello.total",
		metric.WithDescription("HelloWorld requests handled"))
	if err != nil {
		return nil, errors.Wrap(err)
	}
	return &metrics{hello: hello}, nil
}
