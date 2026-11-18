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
	model.Database
	// destructive ops, hidden from model.Database holders
	model.DatabaseDebugger
	// an unreachable server fails Ready, never Alive
	common.Liveness
	common.Readiness
	common.Initializable
	common.Debuggable
}
