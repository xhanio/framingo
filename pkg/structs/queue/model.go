package queue

import (
	"io"
	"xhanio/framingo/pkg/types/common"
)

var (
	_ io.ReadWriter = (DoubleBufferQueue)(nil)
)

type PriorityItem interface {
	common.Unique
	common.Weighted
}

func DefaultLessFunc[T PriorityItem](a, b T) bool {
	priorityDiff := a.GetPriority() - b.GetPriority()
	if priorityDiff != 0 {
		return priorityDiff < 0
	}
	return a.Key() < b.Key()
}

type SimpleQueue[T any] interface {
	Length() int
	IsEmpty() bool
	Push(element ...T)
	Pop() (T, error)
	Reset()
}

type PriorityQueue[T PriorityItem] interface {
	SimpleQueue[T]
	Update(item T) error
	Remove(item T) (T, bool)
	Items() []T
}

type Queue[T any] interface {
	SimpleQueue[T]
	Reset()
	MustGet(i int) T
	MustPop() T
	PopN(n int) ([]T, error)
	Shift() (T, error)
	MustShift() T
	ShiftN(n int) ([]T, error)
}

type DoubleBufferQueueG[T any] interface {
	Write(p []T) (int, error)
	Read(p []T) (int, error)
	io.Closer
}

type DoubleBufferQueue = DoubleBufferQueueG[byte]
