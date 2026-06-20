package supervisor

import (
	"github.com/xhanio/framingo/pkg/types/common"
	"github.com/xhanio/framingo/pkg/types/model"
)

type Manager interface {
	// business
	model.Supervisor
	// lifecycle
	common.Initializable
	common.Daemon
	common.Debuggable
}
