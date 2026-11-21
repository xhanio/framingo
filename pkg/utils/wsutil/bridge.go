package wsutil

import (
	"context"
	"encoding/json"
	"io"
	"sync"
	"sync/atomic"
	"time"

	"github.com/coder/websocket"
	"github.com/coder/websocket/wsjson"
	"github.com/xhanio/errors"

	"github.com/xhanio/framingo/pkg/types/entity"
	"github.com/xhanio/framingo/pkg/types/model"
	"github.com/xhanio/framingo/pkg/utils/log"
)

type bridge struct {
	conn      *websocket.Conn
	messenger model.Messenger

	log          log.Logger
	pingInterval time.Duration
	pingTimeout  time.Duration

	ctx    context.Context
	cancel context.CancelFunc

	once   sync.Once
	ending atomic.Bool // the session is ending: what fails from now on is that
	err    error       // what ended the session, returned for the server to close the connection by
}

// ErrSubscriptionDropped ends a session whose messenger's channel closed under it — the
// bus evicted a subscriber that stopped draining, or pubsub stopped: the connection
// would stay open and hear nothing, so the client is told to try again later.
var ErrSubscriptionDropped = CloseWith(websocket.StatusTryAgainLater, "subscription dropped")

// Bridge relays JSON both ways between conn and messenger, blocking until the session ends,
// and returns what ended it: nil once the connection did, or the error the handler returns
// for the server to close the connection by (wsutil.Close). It closes the Messenger on
// return; conn stays the server's to close.
func Bridge(conn *websocket.Conn, messenger model.Messenger, opts ...Option) error {
	return newBridge(conn, messenger, opts...).run()
}

// ping defaults are seeded before the options since WithPing(0, ...) disables pings
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

func (b *bridge) run() error {
	b.log.Infof("open message session for %s", b.messenger.Name())
	b.ctx, b.cancel = context.WithCancel(context.Background())
	defer b.messenger.Close()
	defer b.cancel()

	go b.pumpOutbound()
	if b.pingInterval > 0 {
		go b.pumpPing()
	}
	b.pumpInbound()
	b.end(nil)
	return b.err
}

// end ends the session for the first reason given. One that closes the connection closes
// it first: canceling the context a read waits on tears the connection down with no
// close frame, and the peer would learn nothing of why. nil just stops the pumps.
func (b *bridge) end(err error) {
	b.once.Do(func() {
		b.ending.Store(true)
		b.err = err
		if err != nil {
			closeBy(b.conn, err) //nolint:errcheck // the server's Close reports err again, once
		}
		b.cancel()
	})
}

func (b *bridge) ended() bool {
	return b.ending.Load()
}

func (b *bridge) pumpInbound() {
	for {
		typ, data, err := b.conn.Read(b.ctx)
		if err != nil {
			switch status := websocket.CloseStatus(err); {
			case status == websocket.StatusNormalClosure, status == websocket.StatusGoingAway:
				b.log.Infof("message session %s closed (status=%d)", b.messenger.Name(), status)
			case errors.Is(err, io.EOF), errors.Is(err, io.ErrUnexpectedEOF):
				// dropped without a close frame: routine, not an error
				b.log.Infof("message session %s disconnected", b.messenger.Name())
			default:
				if !b.ended() {
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
		if err := b.messenger.Send(b.ctx, msg.Kind, msg.Payload); err != nil && !b.ended() {
			b.log.Errorf("failed to relay inbound message from %s: %v", b.messenger.Name(), err)
		}
	}
}

func (b *bridge) pumpOutbound() {
	for {
		select {
		case msg, ok := <-b.messenger.Ch():
			if !ok {
				// the session's own end closes the messenger only after the pumps saw ctx done
				if !b.ended() {
					b.log.Warnf("message session %s lost its subscription", b.messenger.Name())
					b.end(ErrSubscriptionDropped)
				}
				return
			}
			if err := wsjson.Write(b.ctx, b.conn, msg); err != nil {
				if !b.ended() {
					b.log.Errorf("failed to send message through ws for %s: %v", b.messenger.Name(), err)
					b.end(nil) // a connection that takes no write takes no more: the server closes it
				}
				return
			}
		case <-b.ctx.Done():
			return
		}
	}
}

// pumpPing cancels the whole session on a failed ping.
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
				if !b.ended() {
					b.log.Infof("message session %s ping failed: %v", b.messenger.Name(), err)
					b.end(nil)
				}
				return
			}
		}
	}
}
