package task

import (
	"xhanio/framingo/pkg/util/log"
)

type Option func(*task)

func WithLabel(key, val string) Option {
	return func(t *task) {
		t.labels[key] = val
	}
}

func WithLabels(labels map[string]string) Option {
	return func(t *task) {
		t.labels = labels
	}
}

func WithLogger(logger log.Logger) Option {
	return func(t *task) {
		t.log = logger
	}
}
