package platform

import (
	"github.com/xhanio/framingo/pkg/types/model"
)

// Platform bundles the infrastructure business services consume. Every handle
// is always there - the wiring passes each one, never nil - so a consumer calls
// through it unchecked. Tracing stays out: spans arrive through the api middleware.
type Platform interface {
	DB() model.Database
	Meter() model.Meter
	PubSub() model.Pubsub
	MessageBus() model.MessageBus
}
