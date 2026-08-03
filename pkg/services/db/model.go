package db

import (
	"time"

	"github.com/xhanio/framingo/pkg/types/common"
	"github.com/xhanio/framingo/pkg/types/model"
)

type connectionConfig struct {
	MaxOpen     int
	MaxIdle     int
	MaxLifetime time.Duration
	MaxIdleTime time.Duration
	ExecTimeout time.Duration
}

type migrationConfig struct {
	Directory string
	Version   uint
}

type Manager interface {
	// business
	model.Database
	// health.go: Alive guards the manager's own wiring (Init reconnects, so
	// a restart is the remedy); Ready pings the database - an unreachable
	// server is not-ready, never a liveness failure.
	common.Liveness
	common.Readiness
	// lifecycle
	common.Initializable
	common.Debuggable
}
