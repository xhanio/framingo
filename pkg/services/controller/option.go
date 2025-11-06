package controller

import (
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
