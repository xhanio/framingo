package entity

import (
	"time"

	"github.com/xhanio/labels"
)

// PlanKindCommand takes a jobs.Command as Params; its runs report PlanRunStarted/PlanRunFinished.
const PlanKindCommand = "command"

type Plan struct {
	ID          string     `json:"id"`
	Metadata    labels.Set `json:"metadata"`
	Description string     `json:"description"`
	Kind        string     `json:"kind"` // the plan manager builds Task.Job from it
	Params      any        `json:"params,omitempty"`
	Task        *Task      `json:"task"`
}

// PlanRunSource is open: a Trigger caller may define its own values.
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

// PlanRun: each retry or requeue is a new run; a Trigger dropped before it
// starts ends canceled, StartedAt at the Trigger and no log.
type PlanRun struct {
	ID         string        `json:"id"`
	PlanID     string        `json:"plan_id"`
	Source     PlanRunSource `json:"source"`
	State      PlanRunState  `json:"state"`
	StartedAt  time.Time     `json:"started_at"`
	EndedAt    time.Time     `json:"ended_at,omitzero"`
	ExitCode   int           `json:"exit_code"` // for a kind with an exit status, like a command
	OutputTail string        `json:"output_tail,omitempty"`
	LogPath    string        `json:"log_path,omitempty"`
	Error      string        `json:"error,omitempty"`
}

type PlanRunStarted struct {
	PlanRun
}

func (PlanRunStarted) Kind() string {
	return "plan.run.started"
}

type PlanRunFinished struct {
	PlanRun
}

func (PlanRunFinished) Kind() string {
	return "plan.run.finished"
}

type PlanStatsOptions struct {
	Selector string `json:"selector,omitempty"` // over Metadata; empty selects all
	All      bool   `json:"all"`                // include plans whose job succeeded
}

type PlanStats struct {
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
	LastRun       *PlanRun      `json:"last_run,omitempty"` // nil if never run
}
