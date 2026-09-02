package metrics

import (
	"time"

	"github.com/xhanio/framingo/pkg/utils/log"
	"github.com/xhanio/framingo/pkg/utils/nameutil"
)

// defaultInterval matches the SDK PeriodicReader's default cadence - the
// sensible default until an app has a reason to tune it.
const defaultInterval = 60 * time.Second

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
	if m.interval <= 0 {
		m.interval = defaultInterval
	}
}

func WithLogger(logger log.Logger) Option {
	return func(m *manager) {
		m.log = logger
	}
}

func WithName(name string) Option {
	return func(m *manager) {
		m.name = name
	}
}

// WithEnabled switches the service on or off, carrying the wiring layer's
// reading of metrics.enabled - which the same key overrides on every Init,
// since it is also read there. Enabling is its own decision: WithExporter
// says where metrics would go, never that metrics are on.
func WithEnabled(enabled bool) Option {
	return func(m *manager) {
		m.enabled = enabled
	}
}

// WithExporter picks a registry exporter by name together with the config
// block its factory strict-decodes - one decision, since the block only has
// meaning for the exporter that reads it. nil config means the exporter's
// defaults. The programmatic twin of the metrics.exporter + metrics.config
// yaml keys, which override it when set. An exporter whose construction no
// config can express is registered as an app-owned Factory
// (metrics.Register) - a closure captures whatever it needs - and picked
// here by its name.
//
// Choosing where metrics would go does not turn the service on: enabling is
// ops state, not a wiring decision, so metrics.enabled is the only thing
// that does it and there is no option for it.
func WithExporter(name string, config map[string]any) Option {
	return func(m *manager) {
		m.exporterName = name
		m.exporterConfig = config
	}
}

// WithInterval sets the export loop period. Zero reads as unset - newManager
// defaults it - since Go cannot tell an absent duration from an explicit
// zero, which lets a wiring layer pass a missing config key straight
// through. Viper does distinguish unset, so a config file could still ask
// for zero - it lands on the default, never on a spinning loop.
func WithInterval(d time.Duration) Option {
	return func(m *manager) {
		m.interval = d
	}
}
