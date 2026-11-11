package model

import (
	"context"

	"github.com/xhanio/framingo/pkg/types/common"
	"github.com/xhanio/framingo/pkg/types/entity"
)

type Task interface {
	common.Service
	// Add registers all the tasks or, if one of them cannot be, none: a task without a Job
	// is an InvalidArgument, an ID already added a Conflict.
	Add(tasks ...*entity.Task) error
	// Remove forgets the tasks and ends what they are doing, then waits for their executions
	// until ctx ends; nothing of them starts after Remove. What still runs once ctx ends is
	// sent SignalKill, like Stop, and the error wraps ctx's. A task is its job: a copy of a
	// task removes it, another task under its ID does not.
	Remove(ctx context.Context, tasks ...*entity.Task) error
	Trigger(id string, params any) error
	Cancel(id string) error
	Stats(id string) *entity.TaskStats
}
