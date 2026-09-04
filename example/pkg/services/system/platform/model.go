package platform

import (
	"github.com/xhanio/framingo/pkg/types/model"
)

// Platform bundles the framework plumbing business services consume, as
// model.* handles. Each consumer declares its own narrow view of it;
// tracing stays outside - spans arrive through the api middleware.
type Platform interface {
	DB() model.Database
	Meter() model.Meter
	PubSub() model.Pubsub
	MessageBus() model.MessageBus
}
