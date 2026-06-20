package pubsub

import (
	"github.com/xhanio/framingo/pkg/types/common"
	"github.com/xhanio/framingo/pkg/types/model"
)

type Manager interface {
	// business
	model.Pubsub
	// lifecycle
	common.Daemon
	common.Initializable
	common.Debuggable
}
