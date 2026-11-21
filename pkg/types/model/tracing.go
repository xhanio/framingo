package model

import (
	"go.opentelemetry.io/otel/propagation"
	"go.opentelemetry.io/otel/trace"

	"github.com/xhanio/framingo/pkg/types/common"
)

// Tracer is the explicit tracing handle; framingo never uses the otel globals.
type Tracer interface {
	common.Service

	// Provider is never nil (noop when off), and the tracing service's is the
	// same for its life: a restart swaps the pipeline under it, so it and a
	// Tracer from it may be kept.
	Provider() trace.TracerProvider

	Propagator() propagation.TextMapPropagator
}
