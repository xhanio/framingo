package staque

import (
	"github.com/xhanio/framingo/pkg/types/common"
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
