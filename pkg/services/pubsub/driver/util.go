package driver

import (
	"encoding/json"
	"strings"
	"sync"
	"sync/atomic"

	"github.com/xhanio/framingo/pkg/types/entity"
	"github.com/xhanio/framingo/pkg/utils/log"
)

const channelBufferSize = 256

type delivery int

const (
	delivered delivery = iota
	droppedMessage
	// lagging: the caller evicts once it has released the driver lock
	lagging
)

// Publish queues under the driver's read lock; a lock-free pump drains into ch so a slow
// subscriber blocks only its pump. The pump alone closes ch, or it races its own send.
type subscriber struct {
	name string
	ch   chan entity.PubsubMessage

	queueCap int
	onFull   OnFull

	mu      sync.Mutex
	cond    *sync.Cond
	pending []entity.PubsubMessage
	stopped bool
	drops   uint64

	quit     chan struct{}
	done     chan struct{}
	stopOnce sync.Once
	evicting atomic.Bool
}

func newSubscriber(name string, opts *options) *subscriber {
	s := &subscriber{
		name:     name,
		ch:       make(chan entity.PubsubMessage, opts.chanBuf),
		queueCap: opts.queueCap,
		onFull:   opts.onFull,
		quit:     make(chan struct{}),
		done:     make(chan struct{}),
	}
	s.cond = sync.NewCond(&s.mu)
	go s.pump()
	return s
}

// never blocks, so safe under the driver's read lock
func (s *subscriber) offer(msg entity.PubsubMessage) (delivery, uint64) {
	s.mu.Lock()
	if s.stopped {
		s.mu.Unlock()
		return delivered, 0
	}
	if len(s.pending) >= s.queueCap {
		if s.onFull == DropSubscriber {
			s.mu.Unlock()
			return lagging, 0
		}
		s.drops++
		n := s.drops
		s.mu.Unlock()
		return droppedMessage, n
	}
	s.pending = append(s.pending, msg)
	s.mu.Unlock()
	s.cond.Signal()
	return delivered, 0
}

// concurrent publishes can all see a full queue; only one claims the eviction
func (s *subscriber) claimEviction() bool {
	return s.evicting.CompareAndSwap(false, true)
}

func (s *subscriber) pump() {
	defer close(s.done)
	defer close(s.ch)
	for {
		msg, ok := s.next()
		if !ok {
			return
		}
		select {
		case s.ch <- msg:
		case <-s.quit:
			return
		}
	}
}

func (s *subscriber) next() (entity.PubsubMessage, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	for len(s.pending) == 0 && !s.stopped {
		s.cond.Wait()
	}
	if s.stopped {
		return entity.PubsubMessage{}, false
	}
	msg := s.pending[0]
	s.pending[0] = entity.PubsubMessage{}
	s.pending = s.pending[1:]
	return msg, true
}

// idempotent: eviction, Unsubscribe and Stop may all reach it
func (s *subscriber) stop() {
	s.stopOnce.Do(func() {
		s.mu.Lock()
		s.stopped = true
		s.pending = nil
		s.mu.Unlock()
		close(s.quit)
		s.cond.Broadcast()
	})
}

func (s *subscriber) wait() { <-s.done }

func shouldLogDrop(n uint64) bool {
	return n == 1 || n%1024 == 0
}

// topic is the subscribed one, possibly an ancestor of the publish topic
type laggard struct {
	sub   *subscriber
	topic string
}

type dispatcher struct {
	log  log.Logger
	opts *options

	dropped atomic.Uint64
	evicted atomic.Uint64
}

func newDispatcher(logger log.Logger, opts ...Option) *dispatcher {
	return &dispatcher{log: logger, opts: newOptions(opts...)}
}

func (d *dispatcher) Dropped() uint64 { return d.dropped.Load() }

func (d *dispatcher) Evicted() uint64 { return d.evicted.Load() }

// safe under the read lock; a true result must be evicted after releasing it
func (d *dispatcher) offer(sub *subscriber, subTopic string, msg entity.PubsubMessage) bool {
	switch result, drops := sub.offer(msg); result {
	case lagging:
		return true
	case droppedMessage:
		d.dropped.Add(1)
		if shouldLogDrop(drops) {
			d.log.Warnf("pubsub: subscriber %q is not draining %q, dropped %q message (%d dropped so far)",
				sub.name, subTopic, msg.Kind, drops)
		}
	}
	return false
}

// caller holds the read lock and evicts the returned laggards after releasing it
func (d *dispatcher) fanout(topics map[string][]*subscriber, from string, msg entity.PubsubMessage) []laggard {
	var lagged []laggard
	for subTopic, subs := range topics {
		if !topicMatches(subTopic, msg.Topic) {
			continue
		}
		for _, sub := range subs {
			if from != "" && sub.name == from {
				continue
			}
			if d.offer(sub, subTopic, msg) {
				lagged = append(lagged, laggard{sub: sub, topic: subTopic})
			}
		}
	}
	return lagged
}

func (d *dispatcher) claim(l laggard) bool {
	if !l.sub.claimEviction() {
		return false
	}
	d.log.Warnf("pubsub: subscriber %q is not draining %q, evicting it after %d queued messages",
		l.sub.name, l.topic, l.sub.queueCap)
	d.evicted.Add(1)
	return true
}

type eventMessage struct {
	// Origin is the driver instance that published: it delivers locally in
	// Publish and must skip the copy the broker hands back to it
	Origin    string          `json:"origin,omitempty"`
	Publisher string          `json:"publisher"`
	Topic     string          `json:"topic"`
	Kind      string          `json:"kind"`
	Payload   json.RawMessage `json:"payload"`
}

// "app" matches "app", "app/module", "app/module/component"
func topicMatches(subTopic, eventTopic string) bool {
	if subTopic == eventTopic {
		return true
	}
	return strings.HasPrefix(eventTopic, subTopic+"/")
}
