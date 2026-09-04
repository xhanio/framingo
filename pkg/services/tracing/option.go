package tracing

import (
	"github.com/xhanio/framingo/pkg/utils/log"
	"github.com/xhanio/framingo/pkg/utils/nameutil"
)

// defaultSampleRatio traces everything - the sensible default until an app
// has the volume to shed.
const defaultSampleRatio = 1.0

type Option func(*manager)

func (m *manager) applyOptions(opts ...Option) {
	for _, opt := range opts {
		opt(m)
	}
}

func (m *manager) setDefaults() {
	if m.log == nil {
		m.log = log.Default
	}
	if m.name == "" {
		m.name = nameutil.Name(m)
	}
	if m.sampleRatio == 0 {
		m.sampleRatio = defaultSampleRatio
	}
}

func WithLogger(logger log.Logger) Option {
	return func(m *manager) {
		m.log = logger
	}
}

// WithName sets the service name, which also decides the config root Init
// reads the dynamic keys under (nameutil.ConfigRoot): WithName("otel") reads
// otel.enabled, otel.exporter, otel.config and otel.sample_ratio, an unnamed
// instance reads tracing.*.
func WithName(name string) Option {
	return func(m *manager) {
		m.name = name
	}
}

// WithEnabled switches the service on or off, carrying the wiring layer's
// reading of tracing.enabled - which the same key overrides on every Init,
// since it is also read there. Enabling is its own decision: WithExporter
// says where spans would go, never that tracing is on.
func WithEnabled(enabled bool) Option {
	return func(m *manager) {
		m.enabled = enabled
	}
}

// WithExporter picks a registry exporter by name together with the config
// block its factory strict-decodes - one decision, since the block only has
// meaning for the exporter that reads it. nil config means the exporter's
// defaults. The programmatic twin of the tracing.exporter + tracing.config
// yaml keys, which override it when set. An exporter whose construction no
// config can express is registered as an app-owned Factory
// (tracing.Register) - a closure captures whatever it needs - and picked
// here by its name.
//
// Choosing where spans would go does not turn tracing on: enabling is ops
// state, not a wiring decision, so tracing.enabled is the only thing that
// does it and there is no option for it.
func WithExporter(name string, config map[string]any) Option {
	return func(m *manager) {
		m.exporterName = name
		m.exporterConfig = config
	}
}

// WithSampleRatio sets the head sampling ratio, which Init wraps in
// ParentBased so a remote parent's decision still wins. Zero reads as
// unset - newManager defaults it - since Go cannot tell an absent ratio
// from an explicit one, which lets a wiring layer pass a missing config key
// straight through. Viper does distinguish unset, so a config file can
// still ask for zero.
func WithSampleRatio(ratio float64) Option {
	return func(m *manager) {
		m.sampleRatio = ratio
	}
}
