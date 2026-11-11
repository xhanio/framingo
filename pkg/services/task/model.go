package task

import (
	"github.com/xhanio/framingo/pkg/types/common"
	"github.com/xhanio/framingo/pkg/types/model"
)

type Manager interface {
	// business
	model.Task
	// lifecycle
	common.Daemon
}
