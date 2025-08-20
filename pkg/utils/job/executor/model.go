package executor

import (
	"context"
	"time"
)

type Stats struct {
	Retries  uint          `json:"retries"`
	Cooldown time.Duration `json:"cooldown"`
}

type Executor interface {
	Start(ctx context.Context, params any) error
	Stop(wait bool) error
	Stats() *Stats
}
