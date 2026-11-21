package tracing

import (
	"sync"

	"go.opentelemetry.io/otel/propagation"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	"go.opentelemetry.io/otel/trace"

	"github.com/xhanio/framingo/pkg/types/common"
	"github.com/xhanio/framingo/pkg/utils/log"
	"github.com/xhanio/framingo/pkg/utils/traceutil"
)

type manager struct {
	name string
	log  log.Logger

	defaults settings // the built-in defaults as the options set them, fixed at New
	settings settings // what the last Init resolved from the defaults and the config

	// exporter is the tests' seam bypassing the registry; nil means Init builds
	// one. Never written after construction, so read unlocked.
	exporter sdktrace.SpanExporter

	// never swapped, so read unlocked
	propagator propagation.TextMapPropagator

	// mu guards settings.
	mu         sync.RWMutex
	controller *controller

	// what Provider hands out, never swapped: it follows the controller
	provider trace.TracerProvider
}

var _ Manager = (*manager)(nil)

func New(opts ...Option) Manager {
	return newManager(opts...)
}

func newManager(opts ...Option) *manager {
	c := newController()
	m := &manager{
		controller: c,
		provider:   traceutil.Follow(c.current),
		propagator: propagation.NewCompositeTextMapPropagator(propagation.TraceContext{}, propagation.Baggage{}),
	}
	m.applyOptions(opts...)
	m.setDefaults()
	m.settings = m.defaults // what holds until the first Init
	m.log = m.log.By(m)
	return m
}

func (m *manager) Name() string {
	return m.name
}

func (m *manager) Dependencies() []common.Service {
	return nil
}
