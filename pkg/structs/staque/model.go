package staque

import (
	"context"

	"github.com/xhanio/framingo/pkg/types/common"
)

type PriorityItem interface {
	common.Unique
	common.Weighted
}

func DefaultLessFunc[T PriorityItem](a, b T) bool {
	priorityDiff := a.Priority() - b.Priority()
	if priorityDiff != 0 {
		return priorityDiff < 0
	}
	return a.Key() < b.Key()
}

type Stack[T any] interface {
	Length() int
	IsEmpty() bool
	Push(element ...T)
	Pop() (T, error)
	MustPop() T
	Reset()
}

type Queue[T any] interface {
	Length() int
	IsEmpty() bool
	Push(element ...T)
	Shift() (T, error)
	MustShift() T
	Reset()
}

type Priority[T PriorityItem] interface {
	Queue[T]
	Stack[T]
	// WaitPop and WaitShift take an element like Pop and Shift, waiting while the queue is
	// empty. Once ctx ends they return its error and take nothing.
	WaitPop(ctx context.Context) (T, error)
	WaitShift(ctx context.Context) (T, error)
	Update(item T) error
	Remove(item T) (T, bool)
	Items() []T
}

type Simple[T any] interface {
	Queue[T]
	Stack[T]
	PopN(n int) ([]T, error)
	ShiftN(n int) ([]T, error)
}
