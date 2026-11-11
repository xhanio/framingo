package model

import (
	"context"

	"github.com/xhanio/framingo/pkg/types/common"
	"github.com/xhanio/framingo/pkg/types/entity"
)

type TaskManager interface {
	common.Service
	Add(tasks ...*entity.Task) error
	// Remove forgets the tasks and ends what they are doing, then waits for their executions
	// until ctx ends; nothing of them starts after Remove. What still runs once ctx ends is
	// sent SignalKill, like Stop, and the error wraps ctx's.
	Remove(ctx context.Context, tasks ...*entity.Task) error
	Trigger(id string, params any) error
	Cancel(id string) error
	Stats(id string) *entity.TaskStats
}
