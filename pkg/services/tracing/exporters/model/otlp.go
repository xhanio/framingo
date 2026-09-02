// Package model is the tracing factories' config vocabulary: the shared
// exporter core from pkg/utils/otelutil plus the span-exporter-only
// max_request_size knob. The shared package owns Duration, Retry and the
// strict Decode; only the Config wrapper is tracing's own.
package model

import (
	"github.com/xhanio/framingo/pkg/utils/otelutil"
)

// Duration and Retry are the shared vocabulary, re-exported so factory
// configs and tests keep a single import.
type (
	Duration = otelutil.Duration
	Retry    = otelutil.Retry
)

// Config is the transport-agnostic half of the otlp span exporter surface:
// the shared core plus max_request_size, which only the span exporters
// carry.
type Config struct {
	otelutil.Config `yaml:",inline"`
	MaxRequestSize  int `yaml:"max_request_size"`
}

// Decode strict-decodes raw into the factory's config struct: unknown keys
// fail, so a typo'd tracing.config entry surfaces at Init instead of being
// silently ignored. nil or empty raw leaves the struct zero.
func Decode(raw []byte, into any) error {
	return otelutil.Decode(raw, into)
}
