package model

import (
	"context"

	"github.com/xhanio/framingo/pkg/types/common"

	"github.com/xhanio/framingo/example/pkg/types/entity"
)

type Example interface {
	common.Service
	HelloWorld(ctx context.Context, message string) (*entity.HelloWorld, error)
}
