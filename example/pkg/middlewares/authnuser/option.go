package authnuser

import (
	"github.com/xhanio/framingo/pkg/utils/log"
)

type Option func(*middleware)

func WithLogger(logger log.Logger) Option {
	return func(m *middleware) {
		m.log = logger.By(m)
	}
}
