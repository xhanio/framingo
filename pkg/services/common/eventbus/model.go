package eventbus

import (
	"github.com/xhanio/framingo/pkg/types/common"
)

type Manager interface {
	common.Service
	Publish(svc common.Named, topic string, e common.Event)
	Subscribe(svc common.Named, topic string)
}
