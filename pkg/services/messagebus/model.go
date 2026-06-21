package messagebus

import (
	"github.com/xhanio/framingo/pkg/types/common"
	"github.com/xhanio/framingo/pkg/types/model"
)

type Manager interface {
	// business
	model.MessageBus
	// lifecycle
	common.Daemon
	common.Debuggable
}
