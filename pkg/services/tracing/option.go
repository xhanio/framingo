package tracing

import (
	"github.com/xhanio/framingo/pkg/utils/confutil"
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
	if m.defaults.sampleRatio == 0 {
		m.defaults.sampleRatio = defaultSampleRatio
	}
}

// settings are the reloadable ones, re-read by every Init under the config root
// (tracing, or the WithName name).
type settings struct {
	enabled        bool
	exporterName   string
	exporterConfig map[string]any
	sampleRatio    float64
}

// resolve is a start's settings: the defaults, overridden by each key the config
// sets; an invalid value falls back to its default (design/config.md).
func (m *manager) resolve(config confutil.Config) settings {
	s := m.defaults
	root := nameutil.ConfigRoot(m.name)
	s.enabled = config.GetBoolOr(s.enabled, "%s.enabled", root)
	// the block belongs to the exporter in effect: one the config names comes with
	// the config's block alone, the default one takes the config's block if set
	if config.IsSet("%s.exporter", root) {
		s.exporterName = config.GetString("%s.exporter", root)
		s.exporterConfig = config.GetStringMap("%s.config", root)
	} else {
		s.exporterConfig = config.GetStringMapOr(s.exporterConfig, "%s.config", root)
	}
	s.sampleRatio = config.GetFloat64Or(s.sampleRatio, "%s.sample_ratio", root)
	if s.sampleRatio == 0 {
		s.sampleRatio = m.defaults.sampleRatio
	}
	return s
}

// Construction options: fixed for the instance's life, set at New (design/config.md).

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

// Reloadable options: each sets the default of a key Init re-reads at every start;
// where the config sets the key, the config wins (design/config.md).

// WithEnabled sets the default of tracing.enabled (built-in: off).
func WithEnabled(enabled bool) Option {
	return func(m *manager) {
		m.defaults.enabled = enabled
	}
}

// WithExporter sets the default of tracing.exporter and tracing.config (built-in:
// none). A tracing.config alone configures this exporter; a tracing.exporter brings
// its own block. It does not enable tracing.
func WithExporter(name string, config map[string]any) Option {
	return func(m *manager) {
		m.defaults.exporterName = name
		m.defaults.exporterConfig = config
	}
}

// WithSampleRatio sets the default of tracing.sample_ratio (built-in: 1.0; zero is
// unset). A remote parent's sampling decision still wins.
func WithSampleRatio(ratio float64) Option {
	return func(m *manager) {
		m.defaults.sampleRatio = ratio
	}
}
