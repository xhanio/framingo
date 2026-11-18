package metrics

import (
	"context"

	sdkmetric "go.opentelemetry.io/otel/sdk/metric"

	"github.com/xhanio/framingo/pkg/types/common"
	"github.com/xhanio/framingo/pkg/types/model"
)

// Exporter names; each is registered by the matching exporters/ subpackage's init().
const (
	OTLPGRPC = "otlpgrpc"
	OTLPHTTP = "otlphttp"
	Stdout   = "stdout"
)

type Manager interface {
	model.Meter
	// no common.Readiness on purpose: a dead collector must not gate dependents
	common.Liveness
	common.Initializable
	common.Daemon
	common.Debuggable
}

// Factory strict-decodes its own config block (raw YAML, nil when absent) and
// must fail on unknown keys.
type Factory func(ctx context.Context, config []byte) (sdkmetric.Exporter, error)

// Register panics on a duplicate name; call it from init().
func Register(name string, f Factory) {
	reg.Register(name, f)
}
