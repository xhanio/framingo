package common

import (
	"context"
)

type Message interface {
	Kind() string
}

type MessageSender interface {
	Service
	SendMessage(ctx context.Context, from Named, msg Message)
}

type RawMessageSender interface {
	Service
	SendRawMessage(ctx context.Context, from Named, kind string, payload any)
}

// MessageHandler is a module that takes typed messages off the bus. It is the
// module the bus registers, so it is named - the name is its subscription -
// but it need not be a service: nothing is asked of it beyond the name.
type MessageHandler interface {
	Named
	HandleMessage(ctx context.Context, msg Message) error
}

// RawMessageHandler is MessageHandler's catch-all: every message, typed or
// raw, by kind and payload.
type RawMessageHandler interface {
	Named
	HandleRawMessage(ctx context.Context, kind string, payload any) error
}
