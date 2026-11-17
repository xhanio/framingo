package model

import (
	"context"
	"database/sql"
	"time"

	"github.com/xhanio/framingo/pkg/types/common"

	"gorm.io/gorm"
)

// Database is what a consumer of the database holds: connections, queries
// and transactions. It carries nothing that empties or drops the schema -
// those live on DatabaseDebugger.
type Database interface {
	common.Service
	ORM() *gorm.DB
	DB() *sql.DB
	FromContext(ctx context.Context) *gorm.DB
	FromContextTimeout(ctx context.Context, timeout time.Duration) (*gorm.DB, context.CancelFunc)
	// Transaction executes fn within a database transaction.
	Transaction(ctx context.Context, fn func(tctx context.Context) error, opts ...*sql.TxOptions) error
}

// DatabaseDebugger is the destructive side of a database manager, for tests
// and development tooling. It is kept off Database so that nothing holding a
// Database - a repository, a service - can wipe the data; reach it through
// the manager itself (db.Manager, or what testutil.SetupDB returns).
type DatabaseDebugger interface {
	// Cleanup empties every table; with schema, it drops the schema and
	// recreates it empty instead.
	Cleanup(schema bool) error
	// Reload drops the schema and runs the configured migrations again.
	Reload() error
}
