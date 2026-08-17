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

// WithLogger stores the logger raw; New stamps it with By once the name is
// resolved, so a WithName rename shows up in the caller field.
func WithLogger(logger log.Logger) Option {
	return func(m *middleware) {
		m.log = logger
	}
}

// WithName renames the attachment; config keys follow the name, letting two
// differently-named instances run side by side.
func WithName(name string) Option {
	return func(m *middleware) {
		m.name = name
	}
}
