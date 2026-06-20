package repo

import (
	"context"

	"github.com/xhanio/framingo/example/pkg/types/orm"
)

type HelloWorld interface {
	CreateHelloWorld(ctx context.Context, message string) (*orm.HelloWorld, error)
}
