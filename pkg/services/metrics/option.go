package metrics

import (
	"time"

	"github.com/xhanio/framingo/pkg/utils/confutil"
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
	if m.defaults.interval <= 0 {
		m.defaults.interval = defaultInterval
	}
}

// settings are what Init re-reads at every start, under the config root (metrics by
// default, or the WithName name).
type settings struct {
	enabled        bool
	exporterName   string
	exporterConfig map[string]any
	interval       time.Duration
}

// resolve is the settings of a start: the defaults, each key config sets overriding
// one, a value that is not one falling back to its default (design/config.md).
func (m *manager) resolve(config confutil.Config) settings {
	s := m.defaults
	root := nameutil.ConfigRoot(m.name)
	s.enabled = config.GetBoolOr(s.enabled, "%s.enabled", root)
	// a block belongs to the exporter in effect: one the config names takes the
	// config's block alone, never the default exporter's; the default one takes
	// the config's block where it sets one
	if config.IsSet("%s.exporter", root) {
		s.exporterName = config.GetString("%s.exporter", root)
		s.exporterConfig = config.GetStringMap("%s.config", root)
	} else {
		s.exporterConfig = config.GetStringMapOr(s.exporterConfig, "%s.config", root)
	}
	s.interval = config.GetDurationOr(s.interval, "%s.interval", root)
	// zero or negative is unset, so it never spins the loop
	if s.interval <= 0 {
		s.interval = m.defaults.interval
	}
	return s
}

// Construction options: fixed for the instance's life, set at New (design/config.md).

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

// Reloadable options: each sets the default of a key Init re-reads at every start;
// where the config sets the key, the config wins (design/config.md).

// WithEnabled sets the default of metrics.enabled, itself off.
func WithEnabled(enabled bool) Option {
	return func(m *manager) {
		m.defaults.enabled = enabled
	}
}

// WithExporter sets the default of metrics.exporter and metrics.config, itself none; a
// metrics.config alone configures this exporter, a metrics.exporter brings its own
// block. It does not enable the service.
func WithExporter(name string, config map[string]any) Option {
	return func(m *manager) {
		m.defaults.exporterName = name
		m.defaults.exporterConfig = config
	}
}

// WithInterval sets the default of metrics.interval, itself 60s, zero or negative
// being unset.
func WithInterval(d time.Duration) Option {
	return func(m *manager) {
		m.defaults.interval = d
	}
}
