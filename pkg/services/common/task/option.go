package task

import (
	"github.com/xhanio/framingo/pkg/structs/job"
	"github.com/xhanio/framingo/pkg/utils/log"
)

type Option func(*manager)

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

func WithDefaults(opts ...job.Option) Option {
	return func(m *manager) {
		m.defaults = append(m.defaults, opts...)
	}
}
