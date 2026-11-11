package plans

import "github.com/xhanio/framingo/pkg/types/entity"

// Outcome is the part of a PlanRun that the plan's kind decides as one of its runs ends; the
// plan manager fills in the rest. Every kind gives a State and, unless it succeeded, an Error.
type Outcome struct {
	State    entity.PlanRunState
	Error    string // why it failed, or what stopped it
	ExitCode int    // for a kind with an exit status, like a command; 0 for one without
}
