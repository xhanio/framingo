package db

import (
	"time"

	"gorm.io/gorm"

	"github.com/xhanio/framingo/pkg/utils/log"
	"github.com/xhanio/framingo/pkg/utils/nameutil"
)

const (
	defaultMaxOpen     = 10
	defaultMaxIdle     = 5
	defaultMaxLifetime = 5 * time.Minute
	defaultExecTimeout = 30 * time.Second
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
	// MaxIdleTime has no default: zero (never idle out) is a real choice
	if m.connection.MaxOpen == 0 {
		m.connection.MaxOpen = defaultMaxOpen
	}
	if m.connection.MaxIdle == 0 {
		m.connection.MaxIdle = defaultMaxIdle
	}
	if m.connection.MaxLifetime == 0 {
		m.connection.MaxLifetime = defaultMaxLifetime
	}
	if m.connection.ExecTimeout == 0 {
		m.connection.ExecTimeout = defaultExecTimeout
	}
}

func WithLogger(logger log.Logger) Option {
	return func(m *manager) {
		m.log = logger
	}
}

// WithName also sets the config root: <name>.connection.* instead of db.connection.*.
func WithName(name string) Option {
	return func(m *manager) {
		m.name = name
	}
}

func WithType(dbtype string) Option {
	return func(m *manager) {
		m.dbtype = dbtype
	}
}

func WithDataSource(source Source) Option {
	return func(m *manager) {
		m.source = source
	}
}

func WithMigration(sqlDir string, version uint) Option {
	return func(m *manager) {
		m.migration = migrationConfig{
			Directory: sqlDir,
			Version:   version,
		}
	}
}

// WithConnection treats zeros as unset; setDefaults fills them.
func WithConnection(maxOpen int, maxIdle int, maxLifetime time.Duration, maxIdleTime time.Duration, execTimeout time.Duration) Option {
	return func(m *manager) {
		m.connection = connectionConfig{
			MaxOpen:     maxOpen,
			MaxIdle:     maxIdle,
			MaxLifetime: maxLifetime,
			MaxIdleTime: maxIdleTime,
			ExecTimeout: execTimeout,
		}
	}
}

// WithPlugins appends; plugins are reinstalled in order on every connect.
func WithPlugins(plugins ...gorm.Plugin) Option {
	return func(m *manager) {
		m.plugins = append(m.plugins, plugins...)
	}
}
