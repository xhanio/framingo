package model

import (
	"context"

	"github.com/xhanio/framingo/pkg/types/common"
	"github.com/xhanio/framingo/pkg/types/entity"
)

// Pubsub is a bare channel transport; dispatch belongs on top, see MessageBus.
type Pubsub interface {
	common.Service

	// Publish skips subscribers named from. ctx bounds only cross-instance
	// delivery, so a cancel mid-publish can deliver partially.
	Publish(ctx context.Context, from, topic, kind string, payload any) error

	// Subscribe is hierarchical ("app" also gets "app/module"); callers must Unsubscribe.
	Subscribe(name, topic string) (<-chan entity.PubsubMessage, error)

	// Unsubscribe closes the channel.
	Unsubscribe(name, topic string) error
}
