// Package otlpgrpc registers tracing.OTLPGRPC when blank-imported, keeping grpc out of other binaries.
package otlpgrpc

import (
	"context"

	"github.com/xhanio/errors"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracegrpc"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	"google.golang.org/grpc/encoding"

	// registers the gzip compressor that WithCompressor("gzip") selects
	_ "google.golang.org/grpc/encoding/gzip"

	"github.com/xhanio/framingo/pkg/services/tracing"
	"github.com/xhanio/framingo/pkg/services/tracing/exporters/model"
)

func init() {
	tracing.Register(tracing.OTLPGRPC, factory)
}

type config struct {
	model.Config `yaml:",inline"`
}

func factory(ctx context.Context, raw []byte) (sdktrace.SpanExporter, error) {
	var cfg config
	if err := model.Decode(raw, &cfg); err != nil {
		return nil, errors.Wrap(err)
	}
	var opts []otlptracegrpc.Option
	if cfg.Endpoint != "" {
		opts = append(opts, otlptracegrpc.WithEndpoint(cfg.Endpoint))
	}
	if cfg.Insecure {
		opts = append(opts, otlptracegrpc.WithInsecure())
	}
	if len(cfg.Headers) > 0 {
		opts = append(opts, otlptracegrpc.WithHeaders(cfg.Headers))
	}
	if cfg.Compression != "" && cfg.Compression != "none" {
		// fail a typo at Init rather than at export time
		if encoding.GetCompressor(cfg.Compression) == nil {
			return nil, errors.Newf("unsupported otlpgrpc compression: %s", cfg.Compression)
		}
		opts = append(opts, otlptracegrpc.WithCompressor(cfg.Compression))
	}
	if cfg.Timeout > 0 {
		opts = append(opts, otlptracegrpc.WithTimeout(cfg.Timeout.Std()))
	}
	if cfg.MaxRequestSize > 0 {
		opts = append(opts, otlptracegrpc.WithMaxRequestSize(cfg.MaxRequestSize))
	}
	if cfg.Retry != nil {
		opts = append(opts, otlptracegrpc.WithRetry(otlptracegrpc.RetryConfig{
			Enabled:         cfg.Retry.On(),
			InitialInterval: cfg.Retry.InitialInterval.Std(),
			MaxInterval:     cfg.Retry.MaxInterval.Std(),
			MaxElapsedTime:  cfg.Retry.MaxElapsedTime.Std(),
		}))
	}
	exp, err := otlptracegrpc.New(ctx, opts...)
	return exp, errors.Wrap(err)
}
