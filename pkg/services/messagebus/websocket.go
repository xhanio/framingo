package messagebus

import (
	"context"
	"encoding/json"
	"errors"
	"io"

	"github.com/coder/websocket"
	"github.com/coder/websocket/wsjson"

	"github.com/xhanio/framingo/pkg/types/entity"
	"github.com/xhanio/framingo/pkg/types/model"
)

// AttachWebSocket pumps messages between the Messenger and the WebSocket
// connection until the connection closes. It blocks the caller; spawn it in a
// goroutine if you need to do other work in parallel. The Messenger is closed
// on return.
func (m *manager) AttachWebSocket(messenger model.Messenger, ws *websocket.Conn) {
	m.log.Infof("open message session for %s", messenger.Name())

	ctx, cancel := context.WithCancel(context.Background())
	defer messenger.Close()
	defer cancel()

	go m.pumpOutbound(ctx, messenger, ws)

	for {
		typ, data, err := ws.Read(ctx)
		if err != nil {
			switch status := websocket.CloseStatus(err); {
			case status == websocket.StatusNormalClosure, status == websocket.StatusGoingAway:
				m.log.Infof("message session %s closed (status=%d)", messenger.Name(), status)
			case errors.Is(err, io.EOF), errors.Is(err, io.ErrUnexpectedEOF):
				// Client dropped the TCP connection without a close frame
				// (process killed, network drop). Routine, not an error.
				m.log.Infof("message session %s disconnected", messenger.Name())
			default:
				if ctx.Err() == nil {
					m.log.Errorf("message session %s read error: %v", messenger.Name(), err)
				}
			}
			return
		}
		if typ != websocket.MessageText && typ != websocket.MessageBinary {
			continue
		}
		var msg entity.PubsubMessage
		if err := json.Unmarshal(data, &msg); err != nil {
			m.log.Debugf("got unparseable data from %s: %s", messenger.Name(), string(data))
			continue
		}
		if err := messenger.Send(ctx, msg.Kind, msg.Payload); err != nil && ctx.Err() == nil {
			m.log.Errorf("failed to relay inbound message from %s: %v", messenger.Name(), err)
		}
	}
}

func (m *manager) pumpOutbound(ctx context.Context, messenger model.Messenger, ws *websocket.Conn) {
	for {
		select {
		case msg, ok := <-messenger.Ch():
			if !ok {
				return
			}
			if err := wsjson.Write(ctx, ws, msg); err != nil {
				if ctx.Err() == nil {
					m.log.Errorf("failed to send message through ws for %s: %v", messenger.Name(), err)
				}
				return
			}
		case <-ctx.Done():
			return
		}
	}
}
