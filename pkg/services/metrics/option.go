package metrics

import (
	"time"

	"github.com/xhanio/framingo/pkg/utils/log"
	"github.com/xhanio/framingo/pkg/utils/nameutil"
)

// defaultInterval matches the SDK PeriodicReader's default.
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

// WithName also sets the config root Init reads (default metrics.*).
func WithName(name string) Option {
	return func(m *manager) {
		m.name = name
	}
}

// WithEnabled is overridden by metrics.enabled on every Init.
func WithEnabled(enabled bool) Option {
	return func(m *manager) {
		m.enabled = enabled
	}
}

// WithExporter does not enable the service. Init replaces it with
// metrics.exporter and metrics.config, so a key the config leaves out clears it.
func WithExporter(name string, config map[string]any) Option {
	return func(m *manager) {
		m.exporterName = name
		m.exporterConfig = config
	}
}

// WithInterval treats zero as unset, so it never spins the loop.
func WithInterval(d time.Duration) Option {
	return func(m *manager) {
		m.interval = d
	}
}
