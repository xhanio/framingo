package executor

import (
	"time"
	"xhanio/framingo/pkg/types/common"
)

type Stats struct {
	Retries  uint          `json:"retries"`
	Cooldown time.Duration `json:"cooldown"`
}

type Executor interface {
	common.Daemon
	Stats() *Stats
}
