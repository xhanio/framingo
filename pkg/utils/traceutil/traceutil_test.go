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

	// no span in ctx: noop, not a panic or an orphan root span
	_, span = Start(context.Background(), "services/example/manager", "HelloWorld")
	assert.False(t, span.IsRecording())
	span.End()
}

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
