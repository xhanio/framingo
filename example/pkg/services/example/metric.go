package example

import (
	otelmetric "go.opentelemetry.io/otel/metric"
	"go.opentelemetry.io/otel/metric/noop"

	"github.com/xhanio/framingo/pkg/types/model"
)

// metric groups the service's instruments, created once at construction -
// the provider behind model.Meter is stable across restarts.
type metric struct {
	hello otelmetric.Int64Counter
}

// newMetric creates the instruments; a failed registration degrades to noop
// instead of wedging construction.
func (m *manager) newMetric(meter model.Meter) *metric {
	hello, err := meter.Provider().Meter(m.name).Int64Counter("example.hello.total",
		otelmetric.WithDescription("HelloWorld requests handled"))
	if err != nil {
		m.log.Warnf("example.hello.total degraded to noop: %s", err)
		hello, _ = noop.NewMeterProvider().Meter(m.name).Int64Counter("example.hello.total")
	}
	return &metric{hello: hello}
}
