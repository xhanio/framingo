package example

import (
	otelmetric "go.opentelemetry.io/otel/metric"
	"go.opentelemetry.io/otel/metric/noop"

	"github.com/xhanio/framingo/pkg/types/model"
)

// metric groups the service's instruments, created once at construction -
// the provider behind model.Meter is stable for the life of the process, so
// caching them here is safe across service restarts. The struct name
// collides with the otel API package, which is why that import is aliased.
type metric struct {
	hello otelmetric.Int64Counter
}

// newMetric creates the service's instruments, under the service name as the
// instrumentation scope. A failed registration (a duplicate name with a
// different type, an invalid unit) must not wedge construction: the
// instrument degrades to noop with a warning - the otel-recommended
// fallback - and everything else keeps working.
func (m *manager) newMetric(mtr model.Meter) *metric {
	meter := mtr.Provider().Meter(m.name)
	hello, err := meter.Int64Counter("example.hello.total",
		otelmetric.WithDescription("HelloWorld requests handled"))
	if err != nil {
		m.log.Warnf("example.hello.total degraded to noop: %s", err)
		hello, _ = noop.NewMeterProvider().Meter(m.name).Int64Counter("example.hello.total")
	}
	return &metric{hello: hello}
}
