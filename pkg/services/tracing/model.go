package tracing

import (
	"context"

	sdktrace "go.opentelemetry.io/otel/sdk/trace"

	"github.com/xhanio/framingo/pkg/types/common"
	"github.com/xhanio/framingo/pkg/types/model"
)

// Exporter names, the values of the tracing.exporter config key and
// WithExporter. Each maps to a subpackage under pkg/services/tracing/exporters
// that registers its Factory during init().
const (
	OTLPGRPC = "otlpgrpc"
	OTLPHTTP = "otlphttp"
	Stdout   = "stdout"
)

type Manager interface {
	// business
	model.Tracer
	// health.go: Alive guards the manager's own wiring only (enabled but no
	// provider built - Init rebuilds it, so a restart is the remedy). There
	// is deliberately NO common.Readiness: tracing is auxiliary, so an
	// unreachable collector must never gate app readiness - and the monitor
	// rolls a dependency's probes into every dependent's healthcheck, which
	// would let a dead collector degrade the api server.
	common.Liveness
	// lifecycle
	common.Initializable
	common.Daemon
	common.Debuggable
}

// Factory builds a SpanExporter from the tracing.config block, handed over
// as raw YAML - nil when the block is absent. Different exporters need
// different config shapes, so the manager never interprets the block: each
// factory strict-decodes its own config struct and fails the build on
// unknown keys, the same contract api.Middleware.Func has with its config
// bytes. Each exporter subpackage under pkg/services/tracing/exporters/*
// registers one during its init() so the core package never links exporter
// machinery (grpc most of all) into binaries that don't use it. An exporter
// whose construction config cannot express (vendor SDKs, dial options,
// credential objects) is still a Factory: the app Registers its own under a
// name - a closure captures whatever it needs and may ignore config.
type Factory func(ctx context.Context, config []byte) (sdktrace.SpanExporter, error)

// Register makes a Factory available under the given exporter name. It is
// meant to be called from the init() of an exporter subpackage;
// double-registration panics so misconfiguration surfaces at startup.
func Register(name string, f Factory) {
	reg.Register(name, f)
}
