package plan

import (
	"github.com/xhanio/framingo/pkg/types/common"
	"github.com/xhanio/framingo/pkg/types/model"
)

type Manager interface {
	model.Plan
	common.Debuggable
	common.Daemon
}
