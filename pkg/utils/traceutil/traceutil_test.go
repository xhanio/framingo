package traceutil

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
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
