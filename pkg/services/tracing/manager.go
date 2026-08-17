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

	// exporter is an injected instance that bypasses the registry - the
	// package tests' seam, assigned directly on the concrete manager (along
	// with enabled) to observe spans through tracetest. nil means Init
	// builds one from exporterName. Never written after construction time -
	// safe to read without the lock. Production custom exporters go through
	// Register instead: an app-owned Factory captures any construction.
	exporter sdktrace.SpanExporter

	// propagator is fixed at construction and never swapped - safe to read
	// without the lock.
	propagator propagation.TextMapPropagator

	// mu guards the dynamic config fields above (enabled, exporterName,
	// exporterConfig, sampleRatio): Init rewrites them on every restart
	// while Alive and Info keep reading them. The provider pipeline has its
	// own lock inside controller.
	mu         sync.RWMutex
	controller *controller
}

var _ Manager = (*manager)(nil)

func New(opts ...Option) Manager {
	return newManager(opts...)
}

// newManager returns the concrete manager, the form package tests construct.
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
