package driver

import (
	"github.com/xhanio/framingo/pkg/types/common"
	"github.com/xhanio/framingo/pkg/types/entity"
)

// Driver defines the interface for subscription storage and event delivery.
type Driver interface {
	// business

	// Subscribe registers a named subscriber for the given topic and returns
	// a channel that will receive messages published to matching topics.
	Subscribe(name string, topic string) (<-chan entity.PubsubMessage, error)

	// GetSubscribers returns the names of all subscribers matching the given topic,
	// including those subscribed to parent topics.
	GetSubscribers(topic string) []string

	// Unsubscribe removes a subscriber from receiving events for the given topic.
	// The subscriber's channel is closed.
	Unsubscribe(name string, topic string) error

	// Publish dispatches an event to local subscribers and handles
	// cross-instance delivery (e.g., via Redis).
	// The from parameter is the publisher's name, used to skip self-delivery.
	Publish(from string, topic string, kind string, payload any) error

	// lifecycle
	common.Daemon
}
