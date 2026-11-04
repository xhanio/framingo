package executor

import (
	"context"
)

type Stats struct {
	Retries uint `json:"retries"`
}

type Executor interface {
	Start(ctx context.Context, params any) error
	Stop(wait bool) error
	Stats() *Stats
}
