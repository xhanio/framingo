package platform

import (
	"github.com/xhanio/framingo/pkg/types/model"
)

// Platform bundles the framework handles business services consume; tracing
// stays out, spans arrive through the api middleware.
type Platform interface {
	DB() model.Database
	Meter() model.Meter
	PubSub() model.Pubsub
	MessageBus() model.MessageBus
}
