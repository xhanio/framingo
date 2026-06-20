package repository

import (
	"context"
	"database/sql"

	"github.com/xhanio/framingo/pkg/types/common"

	"github.com/xhanio/framingo/example/pkg/types/repo"
)

type Repository interface {
	common.Service
	Transaction(ctx context.Context, fn func(context.Context) error, opts ...*sql.TxOptions) error
	repo.User
	repo.Organization
	repo.Role
	repo.Certificate
	repo.HelloWorld
}
