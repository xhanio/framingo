// Package stdout registers the stdout metric exporter under
// metrics.Stdout. Blank-import it to enable `metrics.exporter: stdout`
// without forcing every binary to carry exporters it doesn't use. It exists
// as the zero-infrastructure exporter for local development.
package stdout

import (
	"context"

	"github.com/xhanio/errors"
	"go.opentelemetry.io/otel/exporters/stdout/stdoutmetric"
	sdkmetric "go.opentelemetry.io/otel/sdk/metric"

	"github.com/xhanio/framingo/pkg/services/metrics"
	"github.com/xhanio/framingo/pkg/utils/otelutil"
)

func init() {
	metrics.Register(metrics.Stdout, factory)
}

// config is this exporter's own shape - nothing like the otlp ones, which is
// exactly why each factory decodes its own struct.
type config struct {
	Pretty     *bool `yaml:"pretty"`     // default true
	Timestamps *bool `yaml:"timestamps"` // default true
}

func factory(_ context.Context, raw []byte) (sdkmetric.Exporter, error) {
	var cfg config
	if err := otelutil.Decode(raw, &cfg); err != nil {
		return nil, errors.Wrap(err)
	}
	var opts []stdoutmetric.Option
	if cfg.Pretty == nil || *cfg.Pretty {
		opts = append(opts, stdoutmetric.WithPrettyPrint())
	}
	if cfg.Timestamps != nil && !*cfg.Timestamps {
		opts = append(opts, stdoutmetric.WithoutTimestamps())
	}
	exp, err := stdoutmetric.New(opts...)
	return exp, errors.Wrap(err)
}
