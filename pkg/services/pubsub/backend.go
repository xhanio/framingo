package pubsub

import (
	"github.com/xhanio/framingo/pkg/types/common"
)

// DispatchFunc is a callback function used by backends to dispatch events to subscribers.
// This allows backends to delegate the actual event delivery logic to the manager.
type DispatchFunc func(subscribers []common.Named, topic string, event common.Event)

// Backend defines the interface for pub/sub subscription management.
// Implementations can be in-memory, Redis-based, or any other message broker.
// The backend is responsible for storing subscriptions and finding matching subscribers,
// but NOT for dispatching events (that's the manager's job).
type Backend interface {
	// Subscribe registers a service to receive events published to the given topic.
	// Topics support hierarchical matching: subscribing to "app" receives events
	// published to "app", "app/module", "app/module/component", etc.
	Subscribe(svc common.Named, topic string) error

	// GetSubscribers returns all subscribers that should receive events for the given topic.
	// For example, publishing to "app/module/component" returns subscribers of:
	// - "app"
	// - "app/module"
	// - "app/module/component"
	GetSubscribers(topic string) []common.Named

	// Unsubscribe removes a service from receiving events for the given topic.
	Unsubscribe(svc common.Named, topic string) error

	// Close cleans up any resources held by the backend.
	Close() error

	// SetDispatcher sets the dispatch function for backends that need to dispatch events
	// (e.g., Redis backend receiving events from other instances).
	SetDispatcher(dispatch DispatchFunc)
}
