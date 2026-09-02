// Package otlpgrpc registers the OTLP-over-gRPC metric exporter under
// metrics.OTLPGRPC. Blank-import it to enable `metrics.exporter: otlpgrpc`
// without forcing every binary to link the grpc stack when it only uses
// other exporters.
package otlpgrpc

import (
	"context"

	"github.com/xhanio/errors"
	"go.opentelemetry.io/otel/exporters/otlp/otlpmetric/otlpmetricgrpc"
	sdkmetric "go.opentelemetry.io/otel/sdk/metric"
	"google.golang.org/grpc/encoding"

	// registers the gzip compressor that WithCompressor("gzip") selects
	_ "google.golang.org/grpc/encoding/gzip"

	"github.com/xhanio/framingo/pkg/services/metrics"
	"github.com/xhanio/framingo/pkg/utils/otelutil"
)

func init() {
	metrics.Register(metrics.OTLPGRPC, factory)
}

// config is this transport's own shape: the shared otlp core and nothing
// else, so http-only keys like url_path fail the strict decode here.
type config struct {
	otelutil.Config `yaml:",inline"`
}

// factory builds the exporter without dialing: otlp connects lazily, so an
// unreachable collector never fails the metrics service's Init.
func factory(ctx context.Context, raw []byte) (sdkmetric.Exporter, error) {
	var cfg config
	if err := otelutil.Decode(raw, &cfg); err != nil {
		return nil, errors.Wrap(err)
	}
	var opts []otlpmetricgrpc.Option
	if cfg.Endpoint != "" {
		opts = append(opts, otlpmetricgrpc.WithEndpoint(cfg.Endpoint))
	}
	if cfg.Insecure {
		opts = append(opts, otlpmetricgrpc.WithInsecure())
	}
	if len(cfg.Headers) > 0 {
		opts = append(opts, otlpmetricgrpc.WithHeaders(cfg.Headers))
	}
	if cfg.Compression != "" && cfg.Compression != "none" {
		// any compressor registered with grpc works (gzip ships
		// blank-imported), but an unregistered name is checked here so a
		// typo fails Init instead of surfacing at export time
		if encoding.GetCompressor(cfg.Compression) == nil {
			return nil, errors.Newf("unsupported otlpgrpc compression: %s", cfg.Compression)
		}
		opts = append(opts, otlpmetricgrpc.WithCompressor(cfg.Compression))
	}
	if cfg.Timeout > 0 {
		opts = append(opts, otlpmetricgrpc.WithTimeout(cfg.Timeout.Std()))
	}
	if cfg.Retry != nil {
		opts = append(opts, otlpmetricgrpc.WithRetry(otlpmetricgrpc.RetryConfig{
			Enabled:         cfg.Retry.On(),
			InitialInterval: cfg.Retry.InitialInterval.Std(),
			MaxInterval:     cfg.Retry.MaxInterval.Std(),
			MaxElapsedTime:  cfg.Retry.MaxElapsedTime.Std(),
		}))
	}
	exp, err := otlpmetricgrpc.New(ctx, opts...)
	return exp, errors.Wrap(err)
}
