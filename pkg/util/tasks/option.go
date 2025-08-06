package tasks

import "xhanio/framingo/pkg/util/task"

type Option func(*manager)

func WithDefaults(opts ...task.Option) Option {
	return func(m *manager) {
		m.defaults = append(m.defaults, opts...)
	}
}
