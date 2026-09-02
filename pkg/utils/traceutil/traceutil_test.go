package traceutil

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/trace"

	"github.com/xhanio/framingo/pkg/utils/testutil"
)

// The reason the type exists: instrumentation libraries resolve Tracer() once,
// at construction, and hold it. A tracing-service restart retires the provider
// behind model.Tracer, and a retired SDK provider drops every span handed to it
// without erroring - so a cached handle goes quiet with nothing to show for it.
func TestLiveProviderResolvesProviderPerSpan(t *testing.T) {
	retiredExp, retiredTP := testutil.NewRecorder()
	tr := testutil.NewTracer(retiredTP)

	// cached before the swap, the way an upstream library caches it
	cached := LiveProvider(tr).Tracer("caller")

	currentExp, currentTP := testutil.NewRecorder()
	tr.Swap(currentTP)
	retiredExp.Reset()

	_, span := cached.Start(context.Background(), "work")
	span.End()

	require.Len(t, currentExp.GetSpans(), 1)
	assert.Equal(t, "work", currentExp.GetSpans()[0].Name)
	assert.Empty(t, retiredExp.GetSpans(), "retired provider must receive nothing")
}

// Delegation happens at two levels, so the scope the caller asked for has to
// survive both hops - otherwise every span is attributed to the empty scope.
func TestLiveProviderForwardsInstrumentationScope(t *testing.T) {
	exp, tp := testutil.NewRecorder()
	tr := testutil.NewTracer(tp)

	_, span := LiveProvider(tr).
		Tracer("my/scope", trace.WithInstrumentationVersion("v1.2.3")).
		Start(context.Background(), "work")
	span.End()

	require.Len(t, exp.GetSpans(), 1)
	scope := exp.GetSpans()[0].InstrumentationScope
	assert.Equal(t, "my/scope", scope.Name)
	assert.Equal(t, "v1.2.3", scope.Version)
}

// Start's contract: the child rides whatever provider made ctx's span, so a
// request traced by the live provider nests correctly, and a bare context
// costs nothing.
func TestStartDerivesTracerFromContext(t *testing.T) {
	exp, tp := testutil.NewRecorder()
	ctx, parent := tp.Tracer("middleware").Start(context.Background(), "HTTP GET")

	ctx, span := Start(ctx, "services/example/manager", "HelloWorld")
	require.True(t, span.IsRecording())
	assert.Equal(t, parent.SpanContext().TraceID(), trace.SpanContextFromContext(ctx).TraceID())
	span.End()
	parent.End()

	spans := exp.GetSpans()
	require.Len(t, spans, 2)
	assert.Equal(t, "HelloWorld", spans[0].Name)
	assert.Equal(t, "services/example/manager", spans[0].InstrumentationScope.Name)
	assert.Equal(t, parent.SpanContext().SpanID(), spans[0].Parent.SpanID())

	// no span in ctx: noop, not a panic and not an orphan root span
	_, span = Start(context.Background(), "services/example/manager", "HelloWorld")
	assert.False(t, span.IsRecording())
	span.End()
}

// Fail's contract: the error lands on the span as event and status, the
// span ends, and the error passes through untouched for the return line.
func TestFailRecordsEndsAndPassesThrough(t *testing.T) {
	exp, tp := testutil.NewRecorder()

	_, span := tp.Tracer("svc").Start(context.Background(), "op")
	require.Same(t, assert.AnError, Fail(span, assert.AnError))

	_, span = tp.Tracer("svc").Start(context.Background(), "op")
	span.End()

	spans := exp.GetSpans()
	require.Len(t, spans, 2)
	assert.Equal(t, codes.Error, spans[0].Status.Code)
	require.Len(t, spans[0].Events, 1, "the error must be recorded as an event")
	assert.Equal(t, "exception", spans[0].Events[0].Name)
	assert.Equal(t, codes.Unset, spans[1].Status.Code)
}
