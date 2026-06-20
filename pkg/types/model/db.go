package model

import (
	"context"
	"database/sql"
	"time"

	"github.com/xhanio/framingo/pkg/types/common"

	"gorm.io/gorm"
)

type Database interface {
	common.Service
	ORM() *gorm.DB
	DB() *sql.DB
	FromContext(ctx context.Context) *gorm.DB
	FromContextTimeout(ctx context.Context, timeout time.Duration) (*gorm.DB, context.CancelFunc)
	Cleanup(schema bool) error
	Reload() error
	// Transaction executes fn within a database transaction.
	Transaction(ctx context.Context, fn func(tctx context.Context) error, opts ...*sql.TxOptions) error
}
