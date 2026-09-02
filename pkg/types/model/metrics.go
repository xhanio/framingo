package model

import (
	"go.opentelemetry.io/otel/metric"

	"github.com/xhanio/framingo/pkg/types/common"
)

// Meter is the explicit metrics handle. Same doctrine as Tracer: framingo
// never touches the otel globals; everything that emits metrics receives
// this handle instead. Only the otel API types appear here — the SDK stays
// behind the metrics service.
type Meter interface {
	common.Service

	// Provider returns the MeterProvider. Always non-nil, and its identity
	// never changes for the life of the process — instruments created from
	// it stay valid across service restarts, so callers create them once
	// (typically at construction) and cache them.
	Provider() metric.MeterProvider
}
