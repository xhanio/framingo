// Package otlphttp registers metrics.OTLPHTTP when blank-imported.
package otlphttp

import (
	"context"

	"github.com/xhanio/errors"
	"go.opentelemetry.io/otel/exporters/otlp/otlpmetric/otlpmetrichttp"
	sdkmetric "go.opentelemetry.io/otel/sdk/metric"

	"github.com/xhanio/framingo/pkg/services/metrics"
	"github.com/xhanio/framingo/pkg/utils/otelutil"
)

func init() {
	metrics.Register(metrics.OTLPHTTP, factory)
}

type config struct {
	otelutil.Config `yaml:",inline"`
	URLPath         string `yaml:"url_path"`
}

func factory(ctx context.Context, raw []byte) (sdkmetric.Exporter, error) {
	var cfg config
	if err := otelutil.Decode(raw, &cfg); err != nil {
		return nil, errors.Wrap(err)
	}
	var opts []otlpmetrichttp.Option
	if cfg.Endpoint != "" {
		opts = append(opts, otlpmetrichttp.WithEndpoint(cfg.Endpoint))
	}
	if cfg.URLPath != "" {
		opts = append(opts, otlpmetrichttp.WithURLPath(cfg.URLPath))
	}
	if cfg.Insecure {
		opts = append(opts, otlpmetrichttp.WithInsecure())
	}
	if len(cfg.Headers) > 0 {
		opts = append(opts, otlpmetrichttp.WithHeaders(cfg.Headers))
	}
	switch cfg.Compression {
	case "", "none":
	case "gzip":
		opts = append(opts, otlpmetrichttp.WithCompression(otlpmetrichttp.GzipCompression))
	default:
		return nil, errors.Newf("unsupported otlphttp compression: %s", cfg.Compression)
	}
	if cfg.Timeout > 0 {
		opts = append(opts, otlpmetrichttp.WithTimeout(cfg.Timeout.Std()))
	}
	if cfg.Retry != nil {
		opts = append(opts, otlpmetrichttp.WithRetry(otlpmetrichttp.RetryConfig{
			Enabled:         cfg.Retry.On(),
			InitialInterval: cfg.Retry.InitialInterval.Std(),
			MaxInterval:     cfg.Retry.MaxInterval.Std(),
			MaxElapsedTime:  cfg.Retry.MaxElapsedTime.Std(),
		}))
	}
	exp, err := otlpmetrichttp.New(ctx, opts...)
	return exp, errors.Wrap(err)
}
