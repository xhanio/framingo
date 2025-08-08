package job

import (
	"xhanio/framingo/pkg/utils/log"
)

type Option func(*job)

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
