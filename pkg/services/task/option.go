package task

import (
	"time"

	"github.com/robfig/cron/v3"

	"github.com/xhanio/framingo/pkg/utils/cronutil"
	"github.com/xhanio/framingo/pkg/utils/log"
	"github.com/xhanio/framingo/pkg/utils/nameutil"
)

const defaultConcurrency = 10

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
	if m.concurrent <= 0 {
		m.concurrent = defaultConcurrency
	}
	if m.loc == nil {
		m.loc = time.Local
	}
	if m.cm == nil {
		m.cm = cron.New(
			cron.WithLocation(m.loc),
			cron.WithParser(cronutil.Parser),
		)
	}
}

func WithLogger(logger log.Logger) Option {
	return func(m *manager) {
		m.log = logger
	}
}

// WithLocation sets the timezone cron schedules are read in; the default is
// time.Local. To schedule by the system's named setting instead, pass what
// osutil.Timezone returns.
func WithLocation(loc *time.Location) Option {
	return func(m *manager) {
		m.loc = loc
	}
}

func MaxConcurrency(size int) Option {
	return func(m *manager) {
		m.concurrent = size
	}
}
