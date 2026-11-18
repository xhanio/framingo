package model

import (
	"context"

	"github.com/xhanio/framingo/pkg/types/common"
	"github.com/xhanio/framingo/pkg/types/entity"
)

// MessageBus routes every message through one well-known Pubsub topic.
type MessageBus interface {
	common.Service

	// Register delivers to module via whichever of MessageHandler and
	// RawMessageHandler it implements.
	Register(module common.Named)

	// senders never receive their own messages
	common.MessageSender

	common.RawMessageSender

	// NewMessenger needs a unique name; the caller must Close the Messenger.
	NewMessenger(name string) (Messenger, error)
}

// Messenger is raw channel access to the bus, sender identity included.
type Messenger interface {
	common.Named

	// Ch is closed by Close.
	Ch() <-chan entity.PubsubMessage

	// Send skips the Messenger itself and returns the pubsub publish error.
	Send(ctx context.Context, kind string, payload any) error

	Close()
}
