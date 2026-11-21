package example

import (
	"github.com/xhanio/framingo/pkg/utils/confutil"
	"github.com/xhanio/framingo/pkg/utils/log"
	"github.com/xhanio/framingo/pkg/utils/nameutil"
)

const defaultGreeting = "hello world!"

type Option func(*manager)

func (m *manager) applyOptions(opts ...Option) {
	for _, opt := range opts {
		opt(m)
	}
}

// setDefaults fills every field the options left zero: the construction fields and the
// defaults of the reloadable ones. It runs after applyOptions, so a zero field means
// the caller set nothing.
func (m *manager) setDefaults() {
	if m.log == nil {
		m.log = log.Default
	}
	if m.name == "" {
		m.name = nameutil.Name(m)
	}
	if m.defaults.greeting == "" {
		m.defaults.greeting = defaultGreeting
	}
}

// settings are the reloadable ones, re-read by every Init under the config root.
type settings struct {
	greeting string
}

// resolve is a start's settings: the defaults, overridden by each key the config
// sets; an invalid value falls back to its default (design/config.md).
func (m *manager) resolve(config confutil.Config) settings {
	s := m.defaults
	s.greeting = config.GetStringOr(s.greeting, "%s.greeting", nameutil.ConfigRoot(m.name))
	if s.greeting == "" {
		s.greeting = m.defaults.greeting
	}
	return s
}

// Construction options: fixed for the instance's life, set at New (design/config.md).

// WithLogger stores the logger raw - newManager stamps it with By once the
// name resolves, so stamping here would bake in an empty caller field.
func WithLogger(logger log.Logger) Option {
	return func(m *manager) {
		m.log = logger
	}
}

// Reloadable options: each sets the default of a key Init re-reads at every start;
// where the config sets the key, the config wins (design/config.md).

// WithGreeting sets the default of example.greeting (built-in: "hello world!").
func WithGreeting(greeting string) Option {
	return func(m *manager) {
		m.defaults.greeting = greeting
	}
}
