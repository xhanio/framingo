package wsutil

import (
	"context"
	"time"

	"github.com/coder/websocket"

	"github.com/xhanio/framingo/pkg/types/entity"
	"github.com/xhanio/framingo/pkg/utils/log"
)

const (
	DefaultPingInterval = 30 * time.Second
	DefaultPingTimeout  = 10 * time.Second
)

type Option func(*session)

func (s *session) applyOptions(opts ...Option) {
	for _, opt := range opts {
		opt(s)
	}
}

// ping settings are seeded by newSession instead, since a zero interval means off
func (s *session) setDefaults() {
	if s.log == nil {
		s.log = log.Default
	}
	if s.filter == nil {
		s.filter = func(entity.PubsubMessage) bool { return true }
	}
	if s.frame == nil {
		s.frame = func(msg entity.PubsubMessage) any { return msg }
	}
	if s.inbound == nil {
		s.inbound = s.relay
	}
}

// Construction options: fixed for the instance's life, set at New (design/config.md).

func WithLogger(logger log.Logger) Option {
	return func(s *session) {
		s.log = logger
	}
}

// WithPing: interval 0 disables pings; a pong missing past timeout ends the session.
func WithPing(interval, timeout time.Duration) Option {
	return func(s *session) {
		s.pingInterval = interval
		s.pingTimeout = timeout
	}
}

// WithFilter writes only the bus messages filter keeps — the session's own, matched
// against its handshake, for instance. By default every message is written.
func WithFilter(filter func(msg entity.PubsubMessage) bool) Option {
	return func(s *session) {
		s.filter = filter
	}
}

// WithFrame writes what frame makes of a bus message, as JSON — its payload alone, for
// instance. By default the message itself is written.
func WithFrame(frame func(msg entity.PubsubMessage) any) Option {
	return func(s *session) {
		s.frame = frame
	}
}

// WithInbound hands each text or binary frame the client sends to inbound — an ack, for
// instance. An error ends the session with that close: a CloseWith's status and reason,
// an internal error for any other. By default a frame is relayed to the bus.
func WithInbound(inbound func(ctx context.Context, data []byte) error) Option {
	return func(s *session) {
		s.inbound = inbound
	}
}

// WithDone ends the session once ch closes, closing with err — a
// CloseWith(StatusNormalClosure, "displaced") for a session a newer one claimed, for
// instance; nil is a normal closure. Given more than once, the first to close ends it.
func WithDone(ch <-chan struct{}, err error) Option {
	if err == nil {
		err = CloseWith(websocket.StatusNormalClosure, "")
	}
	return func(s *session) {
		s.dones = append(s.dones, done{ch: ch, err: err})
	}
}
