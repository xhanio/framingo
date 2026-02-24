package common

import "context"

type Message interface {
	Kind() string
}

type MessageSender interface {
	SendMessage(ctx context.Context, from Named, message Message)
}

type RawMessageSender interface {
	SendRawMessage(ctx context.Context, from Named, kind string, payload any)
}

type MessageHandler interface {
	HandleMessage(ctx context.Context, e Message) error
}

type RawMessageHandler interface {
	HandleRawMessage(ctx context.Context, kind string, payload any) error
}
