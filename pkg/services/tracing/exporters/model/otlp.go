// Package model is the tracing exporter factories' config vocabulary.
package model

import (
	"github.com/xhanio/framingo/pkg/utils/otelutil"
)

type (
	Duration = otelutil.Duration
	Retry    = otelutil.Retry
)

type Config struct {
	otelutil.Config `yaml:",inline"`
	MaxRequestSize  int `yaml:"max_request_size"`
}

// Decode fails on unknown keys; empty raw leaves into zero.
func Decode(raw []byte, into any) error {
	return otelutil.Decode(raw, into)
}
