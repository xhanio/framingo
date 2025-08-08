package task

import "xhanio/framingo/pkg/structs/job"

type Option func(*manager)

func WithDefaults(opts ...job.Option) Option {
	return func(m *manager) {
		m.defaults = append(m.defaults, opts...)
	}
}
