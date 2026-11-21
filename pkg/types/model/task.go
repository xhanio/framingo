package model

import (
	"context"

	"github.com/xhanio/framingo/pkg/types/common"
	"github.com/xhanio/framingo/pkg/types/entity"
)

// Task holds runtime state alone: a stop forgets every task added, so a
// dependent that adds tasks is a common.Reconcilable, and adds them again.
type Task interface {
	common.Service
	// Add is all-or-nothing: no Job is InvalidArgument, a taken ID a Conflict.
	Add(tasks ...*entity.Task) error
	// Remove waits for executions until ctx ends, then sends SignalKill and
	// returns an error wrapping ctx's. It matches by job, not by ID.
	Remove(ctx context.Context, tasks ...*entity.Task) error
	Trigger(id string, params any) error
	Cancel(id string) error
	Stats(id string) *entity.TaskStats
}
