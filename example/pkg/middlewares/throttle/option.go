package throttle

import "github.com/xhanio/framingo/pkg/utils/nameutil"

type Option func(*middleware)

func (m *middleware) applyOptions(opts ...Option) {
	for _, opt := range opts {
		opt(m)
	}
}

func (m *middleware) setDefaults() {
	if m.name == "" {
		m.name = nameutil.Package(m)
	}
}

// Construction options: fixed for the instance's life, set at New (design/config.md).

// WithName also renames the config key, so two instances can run side by side.
func WithName(name string) Option {
	return func(m *middleware) {
		m.name = name
	}
}
