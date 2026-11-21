package db

import (
	"time"

	"gorm.io/gorm"

	"github.com/xhanio/framingo/pkg/utils/confutil"
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
	if m.defaults.MaxOpen == 0 {
		m.defaults.MaxOpen = defaultMaxOpen
	}
	if m.defaults.MaxIdle == 0 {
		m.defaults.MaxIdle = defaultMaxIdle
	}
	if m.defaults.MaxLifetime == 0 {
		m.defaults.MaxLifetime = defaultMaxLifetime
	}
	if m.defaults.ExecTimeout == 0 {
		m.defaults.ExecTimeout = defaultExecTimeout
	}
}

// resolve is the pool of a start, read under db.connection.* (or <name>.connection.*):
// the defaults, each key config sets overriding one, a zero falling back to its
// default (design/config.md).
func (m *manager) resolve(config confutil.Config) connectionConfig {
	c := m.defaults
	root := nameutil.ConfigRoot(m.name)
	c.MaxOpen = config.GetIntOr(c.MaxOpen, "%s.connection.max_open", root)
	c.MaxIdle = config.GetIntOr(c.MaxIdle, "%s.connection.max_idle", root)
	c.MaxLifetime = config.GetDurationOr(c.MaxLifetime, "%s.connection.max_lifetime", root)
	c.MaxIdleTime = config.GetDurationOr(c.MaxIdleTime, "%s.connection.max_idle_time", root)
	c.ExecTimeout = config.GetDurationOr(c.ExecTimeout, "%s.connection.exec_timeout", root)
	// a zero MaxIdleTime is a real choice, never idle out, and stays
	if c.MaxOpen == 0 {
		c.MaxOpen = m.defaults.MaxOpen
	}
	if c.MaxIdle == 0 {
		c.MaxIdle = m.defaults.MaxIdle
	}
	if c.MaxLifetime == 0 {
		c.MaxLifetime = m.defaults.MaxLifetime
	}
	if c.ExecTimeout == 0 {
		c.ExecTimeout = m.defaults.ExecTimeout
	}
	return c
}

// Construction options: fixed for the instance's life, set at New (design/config.md).
// The data source, type and migration among them: a change needs a new process.

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

// WithPlugins appends; plugins are reinstalled in order on every connect.
func WithPlugins(plugins ...gorm.Plugin) Option {
	return func(m *manager) {
		m.plugins = append(m.plugins, plugins...)
	}
}

// Reloadable options: each sets the default of a key Init re-reads at every start;
// where the config sets the key, the config wins (design/config.md).

// WithConnection sets the defaults of db.connection.max_open, max_idle, max_lifetime,
// max_idle_time and exec_timeout, themselves 10, 5, 5m, never and 30s, a zero being
// unset.
func WithConnection(maxOpen int, maxIdle int, maxLifetime time.Duration, maxIdleTime time.Duration, execTimeout time.Duration) Option {
	return func(m *manager) {
		m.defaults = connectionConfig{
			MaxOpen:     maxOpen,
			MaxIdle:     maxIdle,
			MaxLifetime: maxLifetime,
			MaxIdleTime: maxIdleTime,
			ExecTimeout: execTimeout,
		}
	}
}
