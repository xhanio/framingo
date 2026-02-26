package job

import (
	"github.com/xhanio/framingo/pkg/utils/log"
)

type Option func(*job)

func (j *job) apply(opts ...Option) {
	for _, opt := range opts {
		opt(j)
	}
}

func WithLabel(key, val string) Option {
	return func(t *job) {
		t.labels[key] = val
	}
}

func WithLabels(labels map[string]string) Option {
	return func(t *job) {
		t.labels = labels
	}
}

func WithLogger(logger log.Logger) Option {
	return func(t *job) {
		t.log = logger
	}
}
