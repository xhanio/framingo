package example

import (
	"go.opentelemetry.io/otel/metric"
	"go.opentelemetry.io/otel/metric/noop"

	"github.com/xhanio/errors"
)

// instruments groups the service's instruments, built once at Init.
type instruments struct {
	hello metric.Int64Counter
}

// newInstruments creates the instruments; an invalid instrument fails Init
// at first boot instead of degrading silently. The meter is optional - an
// app that wires no metrics service hands the bundle a nil one - and then
// the instruments are noops: the business path records the same way and
// nothing lands, with no nil check where the work happens. Metrics are
// auxiliary, so their absence is a configuration, not a wiring fault.
func (m *manager) newInstruments() (*instruments, error) {
	provider := metric.MeterProvider(noop.NewMeterProvider())
	if m.meter != nil {
		provider = m.meter.Provider()
	}
	hello, err := provider.Meter(m.name).Int64Counter("example.hello.total",
		metric.WithDescription("HelloWorld requests handled"))
	if err != nil {
		return nil, errors.Wrap(err)
	}
	return &instruments{hello: hello}, nil
}
