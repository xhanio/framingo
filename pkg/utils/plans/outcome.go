package plans

import "github.com/xhanio/framingo/pkg/types/entity"

// Outcome always carries a State, and an Error unless it succeeded; the plan manager fills in the rest of the PlanRun.
type Outcome struct {
	State    entity.PlanRunState
	Error    string // why it failed, or what stopped it
	ExitCode int    // for a kind with an exit status, like a command; 0 for one without
}
