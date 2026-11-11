package entity

import (
	"time"

	"k8s.io/apimachinery/pkg/labels"
)

// PlanKindCommand runs a command: its Params is a jobs.Command, the plan manager builds
// its Task.Job, and every run of it is reported with PlanRunStarted and PlanRunFinished.
const PlanKindCommand = "command"

type Plan struct {
	ID          string     `json:"id"`
	Metadata    labels.Set `json:"metadata"`
	Description string     `json:"description"`
	Kind        string     `json:"kind"`             // a predefined kind, from which the plan manager builds the Task's Job
	Params      any        `json:"params,omitempty"` // what the kind takes, as its constant says
	Task        *Task      `json:"task"`
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

// PlanRun is one run of a plan, as of when it was reported. A retry or
// a requeue is a new run. LastRun returns the plan's latest one. A run of Trigger that a
// Cancel, Delete or Stop drops before it starts ends canceled, with the time of its Trigger as
// StartedAt and no log.
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

// PlanRunStarted is sent through the plan manager's MessageSender as a run starts.
type PlanRunStarted struct {
	PlanRun
}

func (PlanRunStarted) Kind() string {
	return "plan.run.started"
}

// PlanRunFinished is sent through the plan manager's MessageSender as a run ends.
type PlanRunFinished struct {
	PlanRun
}

func (PlanRunFinished) Kind() string {
	return "plan.run.finished"
}

// PlanStatsOptions says which plans' stats the plan manager's Stats lists.
type PlanStatsOptions struct {
	Selector string `json:"selector,omitempty"` // a label selector over the plans' Metadata; empty is every plan
	All      bool   `json:"all"`                // also the plans whose job succeeded
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
}
