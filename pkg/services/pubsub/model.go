package pubsub

import (
	"github.com/xhanio/framingo/pkg/types/common"
)

type Manager interface {
	common.Service
	common.Daemon
	common.Initializable
	common.Debuggable
	common.MessageSender
	common.RawMessageSender

	// Publish sends a message to all subscribers of the given topic.
	// The publisher (from) will NOT receive its own message.
	// If payload implements common.Message, MessageHandler subscribers are notified.
	// RawMessageHandler subscribers are always notified.
	Publish(from common.Named, topic string, kind string, payload any)

	// Subscribe registers a service to receive events on the given topic.
	// Topics are hierarchical: subscribing to "app" receives events from
	// "app", "app/module", "app/module/component", etc.
	Subscribe(svc common.Named, topic string)

	// Unsubscribe removes a service's subscription from the given topic.
	Unsubscribe(svc common.Named, topic string)
}
