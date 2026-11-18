package tracing

import (
	"sync"

	"go.opentelemetry.io/otel/propagation"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"

	"github.com/xhanio/framingo/pkg/types/common"
	"github.com/xhanio/framingo/pkg/utils/log"
)

type manager struct {
	name string
	log  log.Logger

	enabled        bool
	exporterName   string
	exporterConfig map[string]any
	sampleRatio    float64

	// exporter is the tests' seam bypassing the registry; nil means Init builds
	// one. Never written after construction, so read unlocked.
	exporter sdktrace.SpanExporter

	// never swapped, so read unlocked
	propagator propagation.TextMapPropagator

	// mu guards enabled, exporterName, exporterConfig and sampleRatio.
	mu         sync.RWMutex
	controller *controller
}

var _ Manager = (*manager)(nil)

func New(opts ...Option) Manager {
	return newManager(opts...)
}

func newManager(opts ...Option) *manager {
	m := &manager{
		controller: newController(),
		propagator: propagation.NewCompositeTextMapPropagator(propagation.TraceContext{}, propagation.Baggage{}),
	}
	m.applyOptions(opts...)
	m.setDefaults()
	m.log = m.log.By(m)
	return m
}

func (m *manager) Name() string {
	return m.name
}

func (m *manager) Dependencies() []common.Service {
	return nil
}
