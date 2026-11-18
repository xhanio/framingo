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

// Tracer's provider can be swapped mid-flight, as a tracing restart does.
type Tracer struct {
	mu       sync.RWMutex
	provider trace.TracerProvider
}

// NewTracer is for tests that swap providers; otherwise tracing.New() needs no setup.
func NewTracer(p trace.TracerProvider) *Tracer {
	return &Tracer{provider: p}
}

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

// NewRecorder's spans are readable the moment they end.
func NewRecorder() (*tracetest.InMemoryExporter, *sdktrace.TracerProvider) {
	exp := tracetest.NewInMemoryExporter()
	return exp, sdktrace.NewTracerProvider(sdktrace.WithSyncer(exp))
}
