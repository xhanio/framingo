package model

import (
	"go.opentelemetry.io/otel/metric"

	"github.com/xhanio/framingo/pkg/types/common"
)

// Meter is the explicit metrics handle; framingo never uses the otel globals.
type Meter interface {
	common.Service

	// Provider is non-nil and fixed for the process lifetime, so instruments
	// may be created once and cached.
	Provider() metric.MeterProvider
}
