// Package traceutil adapts framingo's explicit model.Tracer handle to the
// plain OpenTelemetry interfaces that third-party instrumentation expects.
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

// LiveProvider adapts a model.Tracer into a trace.TracerProvider that
// re-resolves the real provider on every span instead of holding one.
//
// Instrumentation libraries almost all take a TracerProvider and read
// provider.Tracer(...) once, at construction. Handing them model.Tracer's
// provider directly would pin their spans to whichever provider happened to be
// live at wiring time - and a tracing-service restart retires that one, which
// then drops every span it is given without erroring. Use this wherever a
// library wants a TracerProvider:
//
//	tracing.WithTracerProvider(traceutil.LiveProvider(t))
//
// Start resolves through model.Tracer on each call, which costs a lock and a
// map lookup in the SDK's tracer cache. Immaterial at database-statement rates;
// worth measuring before putting it on a per-request path.
func LiveProvider(t model.Tracer) trace.TracerProvider {
	return liveProvider{tracer: t}
}

// liveProvider delegates at both levels, because a caller caches the Tracer as
// well as the provider.
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

// Start begins a child span from the provider already bound to ctx's span -
// the handle-free path for business code. The middleware's request span
// carries the live provider, so no service holds a tracer; a context without
// a recording span (tracing off, unit tests) yields a noop span.
func Start(ctx context.Context, scope, name string, opts ...trace.SpanStartOption) (context.Context, trace.Span) {
	return trace.SpanFromContext(ctx).TracerProvider().Tracer(scope).Start(ctx, name, opts...)
}

// Fail records err on the span, marks it failed, ends it, and hands err
// back - so an error return stays one line:
//
//	return nil, traceutil.Fail(span, errors.Wrap(err))
//
// The success path ends the span itself with a plain span.End(). Every
// return must do one or the other: a path that does neither leaks an
// unended span that never exports.
func Fail(span trace.Span, err error) error {
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
	}
	span.End()
	return err
}
