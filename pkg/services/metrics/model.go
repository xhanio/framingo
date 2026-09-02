package metrics

import (
	"context"

	sdkmetric "go.opentelemetry.io/otel/sdk/metric"

	"github.com/xhanio/framingo/pkg/types/common"
	"github.com/xhanio/framingo/pkg/types/model"
)

// Exporter names, the values of the metrics.exporter config key and
// WithExporter. Each maps to a subpackage under pkg/services/metrics/exporters
// that registers its Factory during init().
const (
	OTLPGRPC = "otlpgrpc"
	OTLPHTTP = "otlphttp"
	Stdout   = "stdout"
)

type Manager interface {
	// business
	model.Meter
	// health.go: Alive guards the manager's own wiring only (enabled but no
	// exporter built - Init rebuilds it, so a restart is the remedy). There
	// is deliberately NO common.Readiness: metrics are auxiliary, so an
	// unreachable collector must never gate app readiness - and the monitor
	// rolls a dependency's probes into every dependent's healthcheck, which
	// would let a dead collector degrade the api server. Export failures
	// never fail Alive either - the monitor restarts on liveness failure,
	// and a dead collector must not flap the service; they surface in logs
	// and Info instead.
	common.Liveness
	// lifecycle
	common.Initializable
	common.Daemon
	common.Debuggable
}

// Factory builds a metric Exporter from the metrics.config block, handed
// over as raw YAML - nil when the block is absent. Different exporters need
// different config shapes, so the manager never interprets the block: each
// factory strict-decodes its own config struct and fails the build on
// unknown keys - the same contract the tracing factories carry. Each
// exporter subpackage under pkg/services/metrics/exporters/* registers one
// during its init() so the core package never links exporter machinery
// (grpc most of all) into binaries that don't use it. An exporter whose
// construction config cannot express is still a Factory: the app Registers
// its own under a name - a closure captures whatever it needs.
type Factory func(ctx context.Context, config []byte) (sdkmetric.Exporter, error)

// Register makes a Factory available under the given exporter name. It is
// meant to be called from the init() of an exporter subpackage;
// double-registration panics so misconfiguration surfaces at startup.
func Register(name string, f Factory) {
	reg.Register(name, f)
}
