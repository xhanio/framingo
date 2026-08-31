package testutil

import (
	"sync"

	"go.opentelemetry.io/otel/propagation"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	"go.opentelemetry.io/otel/sdk/trace/tracetest"
	"go.opentelemetry.io/otel/trace"

	"github.com/xhanio/framingo/pkg/types/common"
	"github.com/xhanio/framingo/pkg/types/model"
)

var _ model.Tracer = (*Tracer)(nil)

// Tracer is a model.Tracer whose provider can be replaced mid-flight, the way
// a tracing-service restart replaces it in production. The provider it hands
// out is whatever the test supplied - pair it with NewRecorder for a real SDK
// provider writing to a real in-memory exporter, so only the service wrapper
// is a double.
type Tracer struct {
	mu       sync.RWMutex
	provider trace.TracerProvider
}

// NewTracer returns a Tracer serving p. Reach for it when a test needs to swap
// the provider mid-flight, which the real tracing service only does through
// Init; a test that just needs some model.Tracer is better served by
// tracing.New(), which serves a noop provider with no Init, config or exporter.
func NewTracer(p trace.TracerProvider) *Tracer {
	return &Tracer{provider: p}
}

// Swap replaces the provider, standing in for a tracing-service restart.
func (t *Tracer) Swap(p trace.TracerProvider) {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.provider = p
}

func (t *Tracer) Name() string                   { return "test-tracer" }
func (t *Tracer) Dependencies() []common.Service { return nil }

func (t *Tracer) Propagator() propagation.TextMapPropagator {
	return propagation.TraceContext{}
}

func (t *Tracer) Provider() trace.TracerProvider {
	t.mu.RLock()
	defer t.mu.RUnlock()
	return t.provider
}

// NewRecorder pairs an in-memory exporter with a synchronous provider, so a
// span is readable the moment it ends - no flush, no sleep, no polling.
func NewRecorder() (*tracetest.InMemoryExporter, *sdktrace.TracerProvider) {
	exp := tracetest.NewInMemoryExporter()
	return exp, sdktrace.NewTracerProvider(sdktrace.WithSyncer(exp))
}
