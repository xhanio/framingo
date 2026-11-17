package repo

import (
	"context"

	"github.com/xhanio/framingo/example/pkg/types/entity"
)

type HelloWorld interface {
	Repository
	CreateHelloWorld(ctx context.Context, message string) (*entity.HelloWorld, error)
}
