package wsutil

import (
	"context"
	"encoding/json"
	"io"
	"time"

	"github.com/coder/websocket"
	"github.com/coder/websocket/wsjson"
	"github.com/xhanio/errors"

	"github.com/xhanio/framingo/pkg/types/entity"
	"github.com/xhanio/framingo/pkg/types/model"
	"github.com/xhanio/framingo/pkg/utils/log"
)

// bridge is one WebSocket connection bridged to a bus Messenger, for as long
// as the connection lasts: the two ends it joins, how it keeps the connection
// alive, and the session's context, whose end stops every pump.
type bridge struct {
	conn      *websocket.Conn
	messenger model.Messenger

	log          log.Logger
	pingInterval time.Duration
	pingTimeout  time.Duration

	ctx    context.Context
	cancel context.CancelFunc
}

// Bridge joins conn to a bus Messenger, both ways, until the connection
// closes: outbound (bus → ws) writes each message the Messenger receives to
// conn as JSON; inbound (ws → bus) decodes each JSON frame read from conn and
// sends it through the Messenger; pings keep the connection alive (WithPing).
// It blocks the caller; spawn it in a goroutine if you need to do other work
// in parallel. The Messenger is closed on return; conn is the caller's to
// close.
func Bridge(conn *websocket.Conn, messenger model.Messenger, opts ...Option) {
	newBridge(conn, messenger, opts...).run()
}

// newBridge seeds the ping settings before the options: a zero interval is
// meaningful - WithPing(0, ...) disables pings - and defaulting it afterwards
// would undo that.
func newBridge(conn *websocket.Conn, messenger model.Messenger, opts ...Option) *bridge {
	b := &bridge{
		conn:         conn,
		messenger:    messenger,
		pingInterval: DefaultPingInterval,
		pingTimeout:  DefaultPingTimeout,
	}
	b.applyOptions(opts...)
	b.setDefaults()
	return b
}

// run is the session: it starts the outbound and ping pumps, takes inbound
// frames until the connection ends, then stops the pumps and closes the
// Messenger.
func (b *bridge) run() {
	b.log.Infof("open message session for %s", b.messenger.Name())
	b.ctx, b.cancel = context.WithCancel(context.Background())
	defer b.messenger.Close()
	defer b.cancel()

	go b.pumpOutbound()
	if b.pingInterval > 0 {
		go b.pumpPing()
	}
	b.pumpInbound()
}

// pumpInbound reads frames until the connection ends - the peer closing or
// dropping it, a read error, or a failed ping canceling the session - and
// sends each JSON message in them through the Messenger.
func (b *bridge) pumpInbound() {
	for {
		typ, data, err := b.conn.Read(b.ctx)
		if err != nil {
			switch status := websocket.CloseStatus(err); {
			case status == websocket.StatusNormalClosure, status == websocket.StatusGoingAway:
				b.log.Infof("message session %s closed (status=%d)", b.messenger.Name(), status)
			case errors.Is(err, io.EOF), errors.Is(err, io.ErrUnexpectedEOF):
				// Client dropped the TCP connection without a close frame
				// (process killed, network drop). Routine, not an error.
				b.log.Infof("message session %s disconnected", b.messenger.Name())
			default:
				if b.ctx.Err() == nil {
					b.log.Errorf("message session %s read error: %v", b.messenger.Name(), err)
				}
			}
			return
		}
		if typ != websocket.MessageText && typ != websocket.MessageBinary {
			continue
		}
		var msg entity.PubsubMessage
		if err := json.Unmarshal(data, &msg); err != nil {
			b.log.Debugf("got unparseable data from %s: %s", b.messenger.Name(), string(data))
			continue
		}
		if err := b.messenger.Send(b.ctx, msg.Kind, msg.Payload); err != nil && b.ctx.Err() == nil {
			b.log.Errorf("failed to relay inbound message from %s: %v", b.messenger.Name(), err)
		}
	}
}

// pumpOutbound writes each message the Messenger receives to the connection
// as JSON, until the session ends or a write fails.
func (b *bridge) pumpOutbound() {
	for {
		select {
		case msg, ok := <-b.messenger.Ch():
			if !ok {
				return
			}
			if err := wsjson.Write(b.ctx, b.conn, msg); err != nil {
				if b.ctx.Err() == nil {
					b.log.Errorf("failed to send message through ws for %s: %v", b.messenger.Name(), err)
				}
				return
			}
		case <-b.ctx.Done():
			return
		}
	}
}

// pumpPing periodically sends WebSocket ping frames to the peer and waits for
// a pong within pingTimeout. On failure (timeout, write error, or the peer
// going away) it cancels the session so the inbound and outbound pumps tear
// down.
func (b *bridge) pumpPing() {
	ticker := time.NewTicker(b.pingInterval)
	defer ticker.Stop()
	for {
		select {
		case <-b.ctx.Done():
			return
		case <-ticker.C:
			pingCtx, pingCancel := context.WithTimeout(b.ctx, b.pingTimeout)
			err := b.conn.Ping(pingCtx)
			pingCancel()
			if err != nil {
				if b.ctx.Err() == nil {
					b.log.Infof("message session %s ping failed: %v", b.messenger.Name(), err)
					b.cancel()
				}
				return
			}
		}
	}
}
