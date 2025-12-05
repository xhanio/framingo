package orm

import (
	"fmt"
)

const NoVersion int64 = 0

type Record[T comparable] interface {
	GetID() T
	GetErased() bool
	GetVersion() int64
	TableName() string
}

type Referenced[T comparable] interface {
	References() []Reference[T]
}

type Reference[T comparable] struct {
	TableName string
	ID        T
}

func (r Reference[T]) Key() string {
	return fmt.Sprintf("%s/%v", r.TableName, r.ID)
}
