// Package traceutil adapts model.Tracer to plain OpenTelemetry interfaces.
package traceutil

import (
	"context"

	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/trace"
	"go.opentelemetry.io/otel/trace/embedded"

	"github.com/xhanio/framingo/pkg/types/model"
)

var (
	_ trace.TracerProvider = liveProvider{}
	_ trace.Tracer         = liveTracer{}
)

// LiveProvider re-resolves the provider on every span, so libraries that cache their tracer at
// construction survive a tracing restart (a retired provider silently drops spans).
// Each Start costs a lock and map lookup; measure before using on a per-request path.
func LiveProvider(t model.Tracer) trace.TracerProvider {
	return liveProvider{tracer: t}
}

// delegates at both levels: callers cache the Tracer as well as the provider
type liveProvider struct {
	embedded.TracerProvider
	tracer model.Tracer
}

func (p liveProvider) Tracer(name string, opts ...trace.TracerOption) trace.Tracer {
	return liveTracer{tracer: p.tracer, name: name, opts: opts}
}

type liveTracer struct {
	embedded.Tracer
	tracer model.Tracer
	name   string
	opts   []trace.TracerOption
}

func (t liveTracer) Start(ctx context.Context, name string, opts ...trace.SpanStartOption) (context.Context, trace.Span) {
	return t.tracer.Provider().Tracer(t.name, t.opts...).Start(ctx, name, opts...)
}

// Start uses the provider bound to ctx's span; without a recording span it yields a noop span.
func Start(ctx context.Context, scope, name string, opts ...trace.SpanStartOption) (context.Context, trace.Span) {
	return trace.SpanFromContext(ctx).TracerProvider().Tracer(scope).Start(ctx, name, opts...)
}

// Fail records a non-nil err, ends the span and returns err; success paths must call span.End() themselves.
func Fail(span trace.Span, err error) error {
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
	}
	span.End()
	return err
}
