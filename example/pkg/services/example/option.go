package example

import (
	"github.com/xhanio/framingo/pkg/utils/log"
)

type Option func(*manager)

func (m *manager) apply(opts ...Option) {
	for _, opt := range opts {
		opt(m)
	}
}

func WithLogger(logger log.Logger) Option {
	return func(m *manager) {
		m.log = logger.By(m)
	}
}

func WithDynamicConfig(greeting string) Option {
	return func(m *manager) {
		m.greeting = greeting
	}
}
