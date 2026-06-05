package model

import (
	"context"
	"database/sql"
	"time"

	"gorm.io/gorm"
)

type DB interface {
	ORM() *gorm.DB
	DB() *sql.DB
	FromContext(ctx context.Context) *gorm.DB
	FromContextTimeout(ctx context.Context, timeout time.Duration) (*gorm.DB, context.CancelFunc)
	Cleanup(schema bool) error
	Reload() error
	// Transaction executes fn within a database transaction.
	Transaction(ctx context.Context, fn func(ctx context.Context) error, opts ...*sql.TxOptions) error
}
