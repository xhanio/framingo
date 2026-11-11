package entity

import (
	"time"

	"k8s.io/apimachinery/pkg/labels"

	"github.com/xhanio/framingo/pkg/utils/task"
)

// PlanKindCommand runs a command: its Params is a CommandParams, the planner builds
// its Task.Job, and every run of it is reported with PlanRunStarted and PlanRunFinished.
const PlanKindCommand = "command"

type Plan struct {
	ID          string     `json:"id"`
	Metadata    labels.Set `json:"metadata"`
	Description string     `json:"description"`
	Kind        string     `json:"kind,omitempty"`   // a predefined kind, or empty for a Task with its own Job
	Params      any        `json:"params,omitempty"` // what the kind takes, as its constant says
	Task        *task.Task `json:"task"`
}

type CommandParams struct {
	Command []string          `json:"command"` // argv, the executable first
	Dir     string            `json:"dir,omitempty"`
	Env     map[string]string `json:"env,omitempty"` // on top of the process environment
}

// PlanRunSource says who started a run; a caller of Trigger may define values of its own.
type PlanRunSource string

const (
	PlanRunSchedule PlanRunSource = "schedule"
	PlanRunManual   PlanRunSource = "manual"
)

type PlanRunState string

const (
	PlanRunRunning   PlanRunState = "running"
	PlanRunSucceeded PlanRunState = "succeeded"
	PlanRunFailed    PlanRunState = "failed"
	PlanRunCanceled  PlanRunState = "canceled"
)

// PlanRun is one run of a plan of a predefined kind, as of when it was reported. A retry or
// a requeue is a new run. LastRun returns the plan's latest one.
type PlanRun struct {
	ID         string        `json:"id"`
	PlanID     string        `json:"plan_id"`
	Source     PlanRunSource `json:"source"`
	State      PlanRunState  `json:"state"`
	StartedAt  time.Time     `json:"started_at"`
	EndedAt    time.Time     `json:"ended_at,omitzero"`
	ExitCode   int           `json:"exit_code"`
	OutputTail string        `json:"output_tail,omitempty"`
	LogPath    string        `json:"log_path,omitempty"`
	Error      string        `json:"error,omitempty"`
}

// PlanRunStarted is sent through the planner's MessageSender as a run starts.
type PlanRunStarted struct {
	PlanRun
}

func (PlanRunStarted) Kind() string {
	return "planner.run.started"
}

// PlanRunFinished is sent through the planner's MessageSender as a run ends.
type PlanRunFinished struct {
	PlanRun
}

func (PlanRunFinished) Kind() string {
	return "planner.run.finished"
}

type PlannerStatsOptions struct {
	Selector string `json:"selector,omitempty"`
	All      bool   `json:"all"`
}

type PlannerStats struct {
	ID            string        `json:"id"`
	Schedule      string        `json:"schedule"`
	State         string        `json:"state"`
	Progress      float64       `json:"progress"`
	StartedAt     time.Time     `json:"started_at"`
	ExecutionTime time.Duration `json:"execution_time"`
	Cooldown      time.Duration `json:"cooldown"`
	Retries       uint          `json:"retries"`
	Labels        labels.Set    `json:"labels"`
	Error         string        `json:"error"`
}
