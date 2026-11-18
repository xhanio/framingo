// Package repo holds the data-access contracts services consume; they speak entity, never orm models or the database.
package repo

import (
	"context"
	"database/sql"

	"github.com/xhanio/framingo/pkg/types/common"
)

// Repository is a service, so consumers list it in Dependencies. Transaction carries one transaction to every
// call made with tctx; an error from fn rolls it back, and one already in ctx is nested on.
type Repository interface {
	common.Service
	Transaction(ctx context.Context, fn func(tctx context.Context) error, opts ...*sql.TxOptions) error
}
