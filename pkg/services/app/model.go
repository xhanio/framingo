package app

import (
	"github.com/xhanio/framingo/pkg/types/common"
	"github.com/xhanio/framingo/pkg/types/model"
)

type Manager interface {
	common.Service
	common.Initializable
	common.Daemon
	common.Debuggable
	model.Application
}
