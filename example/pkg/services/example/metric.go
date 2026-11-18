package example

import (
	"go.opentelemetry.io/otel/metric"
	"go.opentelemetry.io/otel/metric/noop"

	"github.com/xhanio/errors"
)

type instruments struct {
	hello metric.Int64Counter
}

// newInstruments fails Init on an invalid instrument; a nil meter yields noops,
// so the business path never nil-checks.
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
