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

// WithName renames the attachment. The middleware configs key follows the
// name, so a renamed instance activates under its new name - the way out of
// a collision with an app middleware called trace, or the way to run two
// differently-named instances.
func WithName(name string) Option {
	return func(m *middleware) {
		m.name = name
	}
}
