package staque

import (
	"github.com/google/btree"

	"github.com/xhanio/framingo/pkg/utils/log"
)

type Option[T PriorityItem] func(*priority[T])

func WithLessFunc[T PriorityItem](lf btree.LessFunc[T]) Option[T] {
	return func(p *priority[T]) {
		if lf != nil {
			p.lf = lf
		}
	}
}

func WithLogger[T PriorityItem](logger log.Logger) Option[T] {
	return func(p *priority[T]) {
		p.log = logger
	}
}

func BlockIfEmpty[T PriorityItem]() Option[T] {
	return func(p *priority[T]) {
		p.blocking = true
	}
}
