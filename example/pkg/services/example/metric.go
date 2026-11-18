package example

import (
	"go.opentelemetry.io/otel/metric"

	"github.com/xhanio/errors"
)

type instruments struct {
	hello metric.Int64Counter
}

// newInstruments fails Init on an invalid instrument.
func (m *manager) newInstruments() (*instruments, error) {
	hello, err := m.meter.Provider().Meter(m.name).Int64Counter("example.hello.total",
		metric.WithDescription("HelloWorld requests handled"))
	if err != nil {
		return nil, errors.Wrap(err)
	}
	return &instruments{hello: hello}, nil
}
