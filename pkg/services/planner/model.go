package planner

import (
	"github.com/xhanio/framingo/pkg/types/common"
	"github.com/xhanio/framingo/pkg/types/model"
)

type Manager interface {
	common.Service
	common.Debuggable
	common.Daemon
	model.Planner
}
