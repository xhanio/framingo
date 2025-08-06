package common

import "context"

type Event interface {
	Kind() string
}

type EventSender interface {
	SendEvent(ctx context.Context, from Named, message Event)
}

type RawEventSender interface {
	SendRawEvent(ctx context.Context, from Named, kind string, payload any)
}

type EventHandler interface {
	HandleEvent(e Event) error
}

type RawEventHandler interface {
	HandleRawEvent(kind string, payload any) error
}
