package trace

import (
	"github.com/xhanio/framingo/pkg/utils/nameutil"
)

type Option func(*middleware)

func (m *middleware) applyOptions(opts ...Option) {
	for _, opt := range opts {
		opt(m)
	}
}

func (m *middleware) setDefaults() {
	if m.name == "" {
		m.name = nameutil.Package(m) // package name == middleware name
	}
}

// WithName also renames the middleware configs key that activates it.
func WithName(name string) Option {
	return func(m *middleware) {
		m.name = name
	}
}
