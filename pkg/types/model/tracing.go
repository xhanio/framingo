package model

import (
	"go.opentelemetry.io/otel/propagation"
	"go.opentelemetry.io/otel/trace"

	"github.com/xhanio/framingo/pkg/types/common"
)

// Tracer is the explicit tracing handle. Framingo never touches the otel
// globals (otel.SetTracerProvider / otel.SetTextMapPropagator); everything
// that emits or propagates spans receives this handle instead, the same way
// config travels as an instance rather than the viper singleton. Only the
// otel API types appear here — the SDK stays behind the tracing service.
type Tracer interface {
	common.Service

	// Provider returns the active TracerProvider. Always non-nil: a disabled
	// or not-yet-initialized tracing service serves a noop provider, so
	// callers never nil-check and spans cost nothing when tracing is off.
	Provider() trace.TracerProvider

	// Propagator returns the context propagator used to carry trace context
	// across process boundaries (W3C tracecontext + baggage by default).
	Propagator() propagation.TextMapPropagator
}
