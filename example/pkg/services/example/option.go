package example

import (
	"github.com/xhanio/framingo/pkg/services/db"
	"github.com/xhanio/framingo/pkg/utils/log"
)

type Option func(*manager)

func WithLogger(logger log.Logger) Option {
	return func(m *manager) {
		m.log = logger.By(m)
	}
}

func WithDB(database db.Manager) Option {
	return func(m *manager) {
		m.db = database
	}
}
