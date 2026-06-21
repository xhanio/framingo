package model

import (
	"context"

	"github.com/xhanio/framingo/pkg/types/common"
	"github.com/xhanio/framingo/pkg/types/entity"
)

// Pubsub is a topic-routed message transport. It is intentionally minimal:
// publishers send by (from, topic, kind, payload); subscribers receive raw
// channels of entity.PubsubMessage. Higher-level dispatch patterns (typed
// handlers, registered services, websocket bridges) belong to consumers built
// on top — see MessageBus.
type Pubsub interface {
	common.Service

	// Publish dispatches a message to every subscriber matching topic.
	// The from string identifies the publisher and is used to skip self-delivery
	// (subscribers registered under the same name do not receive their own
	// messages). ctx applies only to cross-instance delivery (network hop on
	// the redis/kafka drivers); local fan-out runs to completion regardless,
	// so partial delivery is possible if ctx is canceled mid-publish.
	Publish(ctx context.Context, from, topic, kind string, payload any) error

	// Subscribe registers a named subscriber for the topic and returns a
	// channel that receives every matching message, including sender metadata.
	// Topics are hierarchical: subscribing to "app" receives messages from
	// "app", "app/module", "app/module/component", etc.
	// Callers must call Unsubscribe to release the subscription.
	Subscribe(name, topic string) (<-chan entity.PubsubMessage, error)

	// Unsubscribe removes a subscriber from the topic and closes its channel.
	Unsubscribe(name, topic string) error
}
