package model

import (
	"context"
	"database/sql"
	"time"

	"github.com/xhanio/framingo/pkg/types/common"

	"gorm.io/gorm"
)

// Database deliberately carries nothing destructive; see DatabaseDebugger.
type Database interface {
	common.Service
	ORM() *gorm.DB
	DB() *sql.DB
	FromContext(ctx context.Context) *gorm.DB
	FromContextTimeout(ctx context.Context, timeout time.Duration) (*gorm.DB, context.CancelFunc)
	Transaction(ctx context.Context, fn func(tctx context.Context) error, opts ...*sql.TxOptions) error
}

// DatabaseDebugger is for tests and tooling, reachable only through the manager itself.
type DatabaseDebugger interface {
	// Cleanup empties every table; with schema it drops and recreates the schema.
	Cleanup(schema bool) error
	Reload() error
}
