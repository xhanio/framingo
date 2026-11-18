package testutil

import (
	"go.opentelemetry.io/otel/metric"
	sdkmetric "go.opentelemetry.io/otel/sdk/metric"
	"go.opentelemetry.io/otel/sdk/metric/exemplar"
	"go.opentelemetry.io/otel/sdk/metric/metricdata"

	"github.com/xhanio/framingo/pkg/types/common"
	"github.com/xhanio/framingo/pkg/types/model"
)

var _ model.Meter = (*Meter)(nil)

// Meter has no Swap (unlike Tracer): the metrics service never replaces its provider.
// For any model.Meter without assertions, metrics.New() needs no setup.
type Meter struct {
	provider metric.MeterProvider
}

func NewMeter(p metric.MeterProvider) *Meter {
	return &Meter{provider: p}
}

func (m *Meter) Name() string                   { return "test-meter" }
func (m *Meter) Dependencies() []common.Service { return nil }

func (m *Meter) Provider() metric.MeterProvider {
	return m.provider
}

// NewMeterRecorder returns a provider whose ManualReader collects synchronously.
func NewMeterRecorder() (*sdkmetric.ManualReader, *sdkmetric.MeterProvider) {
	r := sdkmetric.NewManualReader()
	// pinned so OTEL_METRICS_EXEMPLAR_FILTER in the shell cannot affect tests
	return r, sdkmetric.NewMeterProvider(
		sdkmetric.WithReader(r),
		sdkmetric.WithExemplarFilter(exemplar.TraceBasedFilter),
	)
}

// SumInt64 totals all data points; assert found so an absent or mistyped metric is not read as zero.
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

// Histogram reads only the first scope carrying name; assert the bool so an absent or mistyped
// metric is not read as empty.
func Histogram[N int64 | float64](rm metricdata.ResourceMetrics, name string) ([]metricdata.HistogramDataPoint[N], bool) {
	for _, sm := range rm.ScopeMetrics {
		for _, met := range sm.Metrics {
			if met.Name != name {
				continue
			}
			h, ok := met.Data.(metricdata.Histogram[N])
			if !ok {
				return nil, false
			}
			return h.DataPoints, true
		}
	}
	return nil, false
}
