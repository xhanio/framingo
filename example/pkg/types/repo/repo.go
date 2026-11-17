// Package repo holds the data-access contracts services consume, one per
// aggregate. They speak entity: the orm models behind them never cross this
// boundary, so mapping a row to its entity is the repository's job and a
// service never meets a GORM tag - nor the database: services reach
// persistence only through these contracts.
//
// One repository instance implements them all. Each service declares, as
// its own Repository interface in its model.go, the contracts it reads and
// writes through, so what it may touch is named in one place.
package repo

import (
	"context"
	"database/sql"

	"github.com/xhanio/framingo/pkg/types/common"
)

// Repository is what every contract here is built on: a service - so a
// consumer lists the contract in Dependencies - that can run a unit of work.
//
// Transaction runs fn in one database transaction, carried to every
// repository call made with tctx, through whichever contract it goes; an
// error from fn rolls it back. Called with a transaction already in ctx, it
// nests on that one instead of opening another. A service opens one through
// any contract it holds, never through the database.
type Repository interface {
	common.Service
	Transaction(ctx context.Context, fn func(tctx context.Context) error, opts ...*sql.TxOptions) error
}
