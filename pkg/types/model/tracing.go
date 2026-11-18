package model

import (
	"go.opentelemetry.io/otel/propagation"
	"go.opentelemetry.io/otel/trace"

	"github.com/xhanio/framingo/pkg/types/common"
)

// Tracer is the explicit tracing handle; framingo never uses the otel globals.
type Tracer interface {
	common.Service

	// Provider is never nil (noop when off); resolve it per use, a restart replaces it.
	Provider() trace.TracerProvider

	Propagator() propagation.TextMapPropagator
}
