package plan

import (
	"github.com/xhanio/framingo/pkg/utils/log"
	"github.com/xhanio/framingo/pkg/utils/nameutil"
)

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
	if m.tailBytes <= 0 {
		m.tailBytes = defaultTailBytes
	}
}

// WithLogDir: one log file per run, a directory per plan; empty keeps none.
func WithLogDir(dir string) Option {
	return func(m *manager) {
		m.logDir = dir
	}
}

func WithOutputTailBytes(n int) Option {
	return func(m *manager) {
		m.tailBytes = n
	}
}

func WithLogger(logger log.Logger) Option {
	return func(m *manager) {
		m.log = logger
	}
}

func WithName(name string) Option {
	return func(m *manager) {
		m.name = name
	}
}
