package vdb

import (
	"github.com/xhanio/framingo/pkg/utils/log"
)

type Option func(*manager)

func WithLogger(logger log.Logger) Option {
	return func(m *manager) {
		m.log = logger
	}
}

func WithDataSource(source Source) Option {
	return func(m *manager) {
		m.source = source
	}
}
