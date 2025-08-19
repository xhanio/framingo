package db

import (
	"time"

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

func WithType(dbtype string) Option {
	return func(m *manager) {
		m.dbtype = Type(dbtype)
	}
}

func WithDataSource(source Source) Option {
	return func(m *manager) {
		m.source = source
	}
}

func WithMigration(sqlDir string, version uint) Option {
	return func(m *manager) {
		m.migration = MigrationConfig{
			Directory: sqlDir,
			Version:   version,
		}
	}
}

func WithConnection(maxOpen int, maxIdle int, maxLifetime time.Duration, execTimeout time.Duration) Option {
	return func(m *manager) {
		if maxOpen == 0 {
			maxOpen = 10
		}
		if maxIdle == 0 {
			maxIdle = 5
		}
		if maxLifetime == 0 {
			maxLifetime = time.Minute * 5
		}
		if execTimeout == 0 {
			execTimeout = 30 * time.Second
		}
		m.connection = ConnectionConfig{
			MaxOpen:     maxOpen,
			MaxIdle:     maxIdle,
			MaxLifetime: maxLifetime,
			ExecTimeout: execTimeout,
		}
	}
}
