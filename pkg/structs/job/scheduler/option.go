package scheduler

import (
	"time"
	"xhanio/framingo/pkg/utils/log"
)

type Option func(*scheduler)

func MaxConcurrency(size int) Option {
	return func(s *scheduler) {
		s.concurrent = size
	}
}

func WithTimezone(tz *time.Location) Option {
	return func(s *scheduler) {
		s.tz = tz
	}
}

func WithLogger(logger log.Logger) Option {
	return func(s *scheduler) {
		s.log = logger
	}
}
