package tracing

import (
	"github.com/xhanio/framingo/pkg/utils/log"
	"github.com/xhanio/framingo/pkg/utils/nameutil"
)

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

// WithName also sets the config root Init reads (default tracing.*).
func WithName(name string) Option {
	return func(m *manager) {
		m.name = name
	}
}

// WithEnabled is overridden by tracing.enabled on every Init.
func WithEnabled(enabled bool) Option {
	return func(m *manager) {
		m.enabled = enabled
	}
}

// WithExporter does not enable tracing. Init replaces it with tracing.exporter
// and tracing.config, so a key the config leaves out clears it.
func WithExporter(name string, config map[string]any) Option {
	return func(m *manager) {
		m.exporterName = name
		m.exporterConfig = config
	}
}

// WithSampleRatio treats zero as unset; a remote parent's decision still wins.
func WithSampleRatio(ratio float64) Option {
	return func(m *manager) {
		m.sampleRatio = ratio
	}
}
