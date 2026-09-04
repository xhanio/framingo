package repository

import (
	"context"
	"database/sql"

	"github.com/xhanio/framingo/pkg/types/common"
	"github.com/xhanio/framingo/pkg/types/model"

	"github.com/xhanio/framingo/example/pkg/types/repo"
)

// Platform is the repository's narrow view of the app's platform bundle:
// the database handle is all it consumes.
type Platform interface {
	DB() model.Database
}

type Repository interface {
	common.Service
	// health.go
	common.Readiness
	common.Liveness
	Transaction(ctx context.Context, fn func(context.Context) error, opts ...*sql.TxOptions) error
	repo.User
	repo.Organization
	repo.Role
	repo.Certificate
	repo.HelloWorld
}
