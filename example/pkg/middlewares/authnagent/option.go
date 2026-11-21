package authnagent

import (
	"github.com/xhanio/framingo/pkg/utils/log"
	"github.com/xhanio/framingo/pkg/utils/nameutil"
)

type Option func(*middleware)

func (m *middleware) applyOptions(opts ...Option) {
	for _, opt := range opts {
		opt(m)
	}
}

func (m *middleware) setDefaults() {
	if m.log == nil {
		m.log = log.Default
	}
	if m.name == "" {
		m.name = nameutil.Package(m)
	}
}

// Construction options: fixed for the instance's life, set at New (design/config.md).

// WithLogger is stamped in New, after WithName, so a rename shows in the caller field.
func WithLogger(logger log.Logger) Option {
	return func(m *middleware) {
		m.log = logger
	}
}

// WithName also renames the config key, so two instances can run side by side.
func WithName(name string) Option {
	return func(m *middleware) {
		m.name = name
	}
}
