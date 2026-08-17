package feature

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

// WithName renames the attachment; config keys follow the name, letting two
// differently-named instances run side by side.
func WithName(name string) Option {
	return func(m *middleware) {
		m.name = name
	}
}
