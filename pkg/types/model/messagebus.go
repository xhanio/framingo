package model

import (
	"context"

	"github.com/coder/websocket"

	"github.com/xhanio/framingo/pkg/types/common"
	"github.com/xhanio/framingo/pkg/types/entity"
)

// MessageBus is a convenience layer over Pubsub that routes every message
// through a single well-known topic. Modules register once and receive every
// message published on that topic — there are no per-topic subscriptions to
// manage.
//
// Modules participate by implementing common.MessageHandler (typed dispatch)
// and/or common.RawMessageHandler (catch-all). The bus owns the listen loop;
// the underlying pubsub primitive is a pure channel transport.
type MessageBus interface {
	common.Service

	// Register subscribes module to the bus topic. The module receives
	// messages via MessageHandler / RawMessageHandler if it implements them.
	Register(module common.Named)

	// SendMessage publishes a typed message on the bus topic. The sender
	// does not receive its own message.
	common.MessageSender

	// SendRawMessage publishes a message with an arbitrary payload on the bus
	// topic. The sender does not receive its own message.
	common.RawMessageSender

	// NewMessenger creates a Messenger subscribed to the bus topic under the
	// given (unique) name. Use this when you need raw channel access — e.g.,
	// to bridge the bus to a WebSocket, SSE stream, or custom dispatcher.
	// The caller is responsible for calling Close on the returned Messenger.
	NewMessenger(name string) (Messenger, error)

	// AttachWebSocket pumps messages between the Messenger and the WebSocket
	// connection: outbound (bus → ws) writes each message as JSON; inbound
	// (ws → bus) decodes JSON frames and republishes them. The call blocks
	// until the connection closes and then closes the Messenger.
	AttachWebSocket(messenger Messenger, ws *websocket.Conn)
}

// Messenger is a subscriber handle backed by a raw pubsub channel. Unlike
// Register-style modules (which receive via HandleMessage / HandleRawMessage),
// a Messenger gives the caller direct channel access — useful for bridges
// (WebSocket, SSE) that need the full message record including sender identity.
type Messenger interface {
	common.Named

	// Ch returns the receive channel. The channel is closed by Close.
	Ch() <-chan entity.PubsubMessage

	// Send publishes a message on the bus topic with the Messenger as sender.
	// The Messenger does not receive its own message. Returns the error from
	// the underlying pubsub publish (e.g., network failure on a remote driver).
	Send(ctx context.Context, kind string, payload any) error

	// Close unsubscribes from the bus and closes the channel returned by Ch.
	Close()
}
