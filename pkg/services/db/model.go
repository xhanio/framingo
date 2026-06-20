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
	// lifecycle
	common.Initializable
	common.Debuggable
}
