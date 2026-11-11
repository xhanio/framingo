package plan

import (
	"github.com/xhanio/framingo/pkg/types/common"
	"github.com/xhanio/framingo/pkg/types/model"
)

type Manager interface {
	// business
	model.Plan
	// lifecycle
	common.Debuggable
	common.Daemon
}
