package messagebus

import (
	"github.com/xhanio/framingo/pkg/types/common"
	"github.com/xhanio/framingo/pkg/types/model"
)

type Manager interface {
	model.MessageBus
	common.Daemon
	common.Debuggable
	common.Reconcilable
}
