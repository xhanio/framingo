// Package stdout registers the stdout span exporter under
// tracing.Stdout. Blank-import it to enable `tracing.exporter: stdout`
// without forcing every binary to carry exporters it doesn't use. It exists
// as the zero-infrastructure exporter for local development.
package stdout

import (
	"context"

	"github.com/xhanio/errors"
	"go.opentelemetry.io/otel/exporters/stdout/stdouttrace"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"

	"github.com/xhanio/framingo/pkg/services/tracing"
	"github.com/xhanio/framingo/pkg/services/tracing/exporters/model"
)

func init() {
	tracing.Register(tracing.Stdout, factory)
}

// config is this exporter's own shape - nothing like the otlp ones, which is
// exactly why each factory decodes its own struct.
type config struct {
	Pretty     *bool `yaml:"pretty"`     // default true
	Timestamps *bool `yaml:"timestamps"` // default true
}

func factory(_ context.Context, raw []byte) (sdktrace.SpanExporter, error) {
	var cfg config
	if err := model.Decode(raw, &cfg); err != nil {
		return nil, errors.Wrap(err)
	}
	var opts []stdouttrace.Option
	if cfg.Pretty == nil || *cfg.Pretty {
		opts = append(opts, stdouttrace.WithPrettyPrint())
	}
	if cfg.Timestamps != nil && !*cfg.Timestamps {
		opts = append(opts, stdouttrace.WithoutTimestamps())
	}
	exp, err := stdouttrace.New(opts...)
	return exp, errors.Wrap(err)
}
