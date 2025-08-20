package task

import (
	"github.com/xhanio/framingo/pkg/utils/log"
)

type Option func(*manager)

func WithLogger(logger log.Logger) Option {
	return func(m *manager) {
		m.log = logger
	}
}

func MaxConcurrency(size int) Option {
	return func(m *manager) {
		m.concurrent = size
	}
}
