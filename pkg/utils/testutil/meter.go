package testutil

import (
	"go.opentelemetry.io/otel/metric"
	sdkmetric "go.opentelemetry.io/otel/sdk/metric"
	"go.opentelemetry.io/otel/sdk/metric/metricdata"

	"github.com/xhanio/framingo/pkg/types/common"
	"github.com/xhanio/framingo/pkg/types/model"
)

var _ model.Meter = (*Meter)(nil)

// Meter is a model.Meter serving whatever provider the test supplied. There
// is deliberately no Swap counterpart to testutil.Tracer's: the production
// metrics service never replaces its provider, so there is no mid-flight
// swap for a test to imitate. Pair it with NewMeterRecorder for a real SDK
// provider whose collected state a test reads back synchronously; a test
// that just needs some model.Meter is better served by metrics.New(), which
// serves a real never-exporting provider with no Init, config or exporter.
type Meter struct {
	provider metric.MeterProvider
}

// NewMeter returns a Meter serving p.
func NewMeter(p metric.MeterProvider) *Meter {
	return &Meter{provider: p}
}

func (m *Meter) Name() string                   { return "test-meter" }
func (m *Meter) Dependencies() []common.Service { return nil }

func (m *Meter) Provider() metric.MeterProvider {
	return m.provider
}

// NewMeterRecorder pairs a ManualReader with a real SDK provider, so a test
// creates instruments, records, then Collects and asserts on metricdata -
// no flush, no sleep, no polling.
func NewMeterRecorder() (*sdkmetric.ManualReader, *sdkmetric.MeterProvider) {
	r := sdkmetric.NewManualReader()
	return r, sdkmetric.NewMeterProvider(sdkmetric.WithReader(r))
}

// SumInt64 digs the int64 Sum total of the named instrument out of one
// collected export, summed across data points. found reports whether the
// instrument appeared with that shape at all - assert on it, so an absent
// or mistyped metric fails the test instead of reading as zero.
func SumInt64(rm metricdata.ResourceMetrics, name string) (total int64, found bool) {
	for _, sm := range rm.ScopeMetrics {
		for _, met := range sm.Metrics {
			if met.Name != name {
				continue
			}
			sum, ok := met.Data.(metricdata.Sum[int64])
			if !ok {
				return 0, false
			}
			for _, dp := range sum.DataPoints {
				total += dp.Value
			}
			return total, true
		}
	}
	return 0, false
}
