package supervisor

import (
	"github.com/xhanio/framingo/pkg/types/common"
	"github.com/xhanio/framingo/pkg/types/model"
)

type Manager interface {
	// business
	model.Supervisor
	// health.go: Alive fails only when in-process recovery is spent (a
	// service dead with restarts exhausted) - the signal to let the platform
	// replace the process. Ready is the roll-up: nil iff every registered
	// service is ready. Both judge from cached stats; probing stays the
	// monitor's job.
	common.Liveness
	common.Readiness
	// lifecycle
	common.Initializable
	common.Daemon
	common.Debuggable
}
