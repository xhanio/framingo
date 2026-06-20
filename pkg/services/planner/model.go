package planner

import (
	"github.com/xhanio/framingo/pkg/types/common"
	"github.com/xhanio/framingo/pkg/types/model"
)

type Manager interface {
	// business
	model.Planner
	// lifecycle
	common.Debuggable
	common.Daemon
}
