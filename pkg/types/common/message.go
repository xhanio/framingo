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

// MessageHandler's name is its subscription; it need not be a Service.
type MessageHandler interface {
	Named
	HandleMessage(ctx context.Context, msg Message) error
}

// RawMessageHandler receives every message, typed or raw.
type RawMessageHandler interface {
	Named
	HandleRawMessage(ctx context.Context, kind string, payload any) error
}
