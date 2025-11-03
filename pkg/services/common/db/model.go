package db

import (
	"context"
	"database/sql"
	"time"

	"gorm.io/gorm"

	"github.com/xhanio/framingo/pkg/types/common"
)

type ConnectionConfig struct {
	MaxOpen     int
	MaxIdle     int
	MaxLifetime time.Duration
	ExecTimeout time.Duration
}

type MigrationConfig struct {
	Directory string
	Version   uint
}

type Manager interface {
	common.Service
	common.Initializable
	common.Debuggable
	ORM() *gorm.DB
	DB() *sql.DB
	FromContext(ctx context.Context) *gorm.DB
	FromContextTimeout(ctx context.Context, timeout time.Duration) (*gorm.DB, context.CancelFunc)
}
