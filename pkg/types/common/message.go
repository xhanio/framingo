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

type MessageHandler interface {
	Service
	HandleMessage(ctx context.Context, msg Message) error
}

type RawMessageHandler interface {
	Service
	HandleRawMessage(ctx context.Context, kind string, payload any) error
}
