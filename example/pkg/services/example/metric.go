package example

import (
	"go.opentelemetry.io/otel/metric"

	"github.com/xhanio/errors"
)

// instruments groups the service's instruments, built once at Init.
type instruments struct {
	hello metric.Int64Counter
}

// newInstruments creates the instruments; an invalid instrument fails Init
// at first boot instead of degrading silently.
func (m *manager) newInstruments() (*instruments, error) {
	hello, err := m.meter.Provider().Meter(m.name).Int64Counter("example.hello.total",
		metric.WithDescription("HelloWorld requests handled"))
	if err != nil {
		return nil, errors.Wrap(err)
	}
	return &instruments{hello: hello}, nil
}
