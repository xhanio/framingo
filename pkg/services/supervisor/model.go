package supervisor

import (
	"github.com/xhanio/framingo/pkg/types/common"
	"github.com/xhanio/framingo/pkg/types/model"
)

type Manager interface {
	model.Supervisor
	// Alive fails only once in-process recovery is spent; Ready is nil iff every
	// service is ready. Both read cached stats, never probe.
	common.Liveness
	common.Readiness
	common.Initializable
	common.Daemon
	common.Debuggable
}
