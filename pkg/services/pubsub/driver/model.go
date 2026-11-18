package driver

import (
	"context"

	"github.com/xhanio/framingo/pkg/types/common"
	"github.com/xhanio/framingo/pkg/types/entity"
)

type Driver interface {
	// Subscribe's channel also receives messages published to descendant topics.
	Subscribe(name string, topic string) (<-chan entity.PubsubMessage, error)

	GetSubscribers(topic string) []string

	// Unsubscribe closes the subscriber's channel.
	Unsubscribe(name string, topic string) error

	// Publish skips self-delivery to from; ctx bounds only the cross-instance hop, so delivery can be partial.
	Publish(ctx context.Context, from string, topic string, kind string, payload any) error

	common.Daemon
}
