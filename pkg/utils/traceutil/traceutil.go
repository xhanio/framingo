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

// Follow is a provider that resolves current on every span, so a Tracer kept
// from it follows whatever current returns then: how the tracing service's
// Provider stays the same across a restart that swaps its pipeline. Each Start
// costs current and the provider's own Tracer lookup.
func Follow(current func() trace.TracerProvider) trace.TracerProvider {
	return liveProvider{current: current}
}

// LiveProvider follows t's Provider, for a model.Tracer that replaces it; the
// tracing service's own Provider follows its restarts already.
func LiveProvider(t model.Tracer) trace.TracerProvider {
	return Follow(t.Provider)
}

// delegates at both levels: callers cache the Tracer as well as the provider
type liveProvider struct {
	embedded.TracerProvider
	current func() trace.TracerProvider
}

func (p liveProvider) Tracer(name string, opts ...trace.TracerOption) trace.Tracer {
	return liveTracer{current: p.current, name: name, opts: opts}
}

type liveTracer struct {
	embedded.Tracer
	current func() trace.TracerProvider
	name    string
	opts    []trace.TracerOption
}

func (t liveTracer) Start(ctx context.Context, name string, opts ...trace.SpanStartOption) (context.Context, trace.Span) {
	return t.current().Tracer(t.name, t.opts...).Start(ctx, name, opts...)
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
