package wsutil

import (
	"context"
	"encoding/json"
	"sync"
	"sync/atomic"
	"time"

	"github.com/coder/websocket"
	"github.com/coder/websocket/wsjson"

	"github.com/xhanio/framingo/pkg/types/entity"
	"github.com/xhanio/framingo/pkg/types/model"
	"github.com/xhanio/framingo/pkg/utils/log"
)

// ErrSubscriptionDropped ends a session whose messenger's channel closed — the bus
// evicted a subscriber that stopped draining, or pubsub stopped — telling the client
// to try again later instead of leaving it on a connection that hears nothing.
var ErrSubscriptionDropped = CloseWith(websocket.StatusTryAgainLater, "subscription dropped")

// Bridge relays JSON both ways between conn and messenger until the session ends, and
// returns what the handler returns: nil if the connection went first, else the close
// the session sent — a dropped subscription, a WithDone, an inbound error. It closes
// the messenger; conn stays the server's to close.
func Bridge(conn *websocket.Conn, messenger model.Messenger, opts ...Option) error {
	return newSession(conn, messenger, opts...).run()
}

// session is one connection bridged to one messenger: its read and write loops, its
// ping, and the ends it watches for.
type session struct {
	conn      *websocket.Conn
	messenger model.Messenger

	log          log.Logger
	pingInterval time.Duration
	pingTimeout  time.Duration
	filter       func(entity.PubsubMessage) bool     // which bus messages are written
	frame        func(entity.PubsubMessage) any      // what is written for one
	inbound      func(context.Context, []byte) error // what a client frame does
	dones        []done

	ctx    context.Context
	cancel context.CancelFunc

	once   sync.Once
	over   atomic.Bool // end has begun: what fails from then on is the session ending
	reason error       // the close end sent, if any, returned by Bridge
}

// done is a WithDone: the session ends with err once ch closes.
type done struct {
	ch  <-chan struct{}
	err error
}

// ping defaults are seeded before the options since WithPing(0, ...) disables pings
func newSession(conn *websocket.Conn, messenger model.Messenger, opts ...Option) *session {
	s := &session{
		conn:         conn,
		messenger:    messenger,
		pingInterval: DefaultPingInterval,
		pingTimeout:  DefaultPingTimeout,
	}
	s.applyOptions(opts...)
	s.setDefaults()
	return s
}

func (s *session) run() error {
	s.log.Infof("session %s: open", s.messenger.Name())
	s.ctx, s.cancel = context.WithCancel(context.Background())
	defer s.messenger.Close()
	defer s.cancel()

	go s.writeLoop()
	if s.pingInterval > 0 {
		go s.pingLoop()
	}
	for _, d := range s.dones {
		go s.watch(d)
	}
	s.readLoop()
	s.end(nil)
	return s.reason
}

// end ends the session, once. Given a reason it sends that close before stopping the
// loops, since canceling a read's context tears the connection down with no close
// frame; the reason is returned marked sent, so the server's Close does not send it
// again. Given nil, the connection is gone already and the loops just stop.
func (s *session) end(reason error) {
	s.once.Do(func() {
		s.over.Store(true)
		if reason != nil {
			closeAs(s.conn, reason) //nolint:errcheck // the server's Close reports it, once
			s.reason = sent(reason)
		}
		s.cancel()
	})
}

// ending reports whether end has begun: a failure from then on is the session ending.
func (s *session) ending() bool {
	return s.over.Load()
}

// readLoop hands each frame the client sends to inbound, until the connection goes.
func (s *session) readLoop() {
	for {
		typ, data, err := s.conn.Read(s.ctx)
		if err != nil {
			switch {
			case s.ending():
			case PeerGone(err):
				s.log.Infof("session %s: client gone: %v", s.messenger.Name(), err)
			default:
				s.log.Errorf("session %s: read failed: %v", s.messenger.Name(), err)
			}
			return
		}
		if typ != websocket.MessageText && typ != websocket.MessageBinary {
			continue
		}
		if err := s.inbound(s.ctx, data); err != nil {
			s.end(err)
			return
		}
	}
}

// writeLoop writes the frame of each bus message the filter keeps.
func (s *session) writeLoop() {
	for {
		select {
		case <-s.ctx.Done():
			return
		case msg, ok := <-s.messenger.Ch():
			if !ok {
				// the session's own end closes the messenger only after the loops stopped
				if !s.ending() {
					s.log.Warnf("session %s: subscription dropped", s.messenger.Name())
					s.end(ErrSubscriptionDropped)
				}
				return
			}
			if !s.filter(msg) {
				continue
			}
			if err := wsjson.Write(s.ctx, s.conn, s.frame(msg)); err != nil {
				if !s.ending() {
					s.log.Errorf("session %s: write failed: %v", s.messenger.Name(), err)
					s.end(nil) // a connection that takes no write is gone
				}
				return
			}
		}
	}
}

// pingLoop ends the session once a pong is missing past the timeout.
func (s *session) pingLoop() {
	ticker := time.NewTicker(s.pingInterval)
	defer ticker.Stop()
	for {
		select {
		case <-s.ctx.Done():
			return
		case <-ticker.C:
			ctx, cancel := context.WithTimeout(s.ctx, s.pingTimeout)
			err := s.conn.Ping(ctx)
			cancel()
			if err != nil {
				if !s.ending() {
					s.log.Infof("session %s: ping failed: %v", s.messenger.Name(), err)
					s.end(nil)
				}
				return
			}
		}
	}
}

// watch ends the session with d's close once d.ch closes.
func (s *session) watch(d done) {
	select {
	case <-s.ctx.Done():
	case <-d.ch:
		s.end(d.err)
	}
}

// relay is the default inbound: it sends a frame that parses as a PubsubMessage on the
// bus, as its kind and payload, and skips one that does not.
func (s *session) relay(ctx context.Context, data []byte) error {
	var msg entity.PubsubMessage
	if err := json.Unmarshal(data, &msg); err != nil {
		s.log.Debugf("session %s: skipped unparseable frame: %s", s.messenger.Name(), string(data))
		return nil
	}
	if err := s.messenger.Send(ctx, msg.Kind, msg.Payload); err != nil && !s.ending() {
		s.log.Errorf("session %s: relay failed: %v", s.messenger.Name(), err)
	}
	return nil
}
