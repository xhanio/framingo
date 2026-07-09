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

// delivery is the outcome of offering a message to a subscriber.
type delivery int

const (
	// delivered means the message was queued for the subscriber.
	delivered delivery = iota
	// droppedMessage means the queue was full and the message was discarded.
	droppedMessage
	// lagging means the queue was full and the subscriber must be evicted by
	// the caller, once it has released the driver lock.
	lagging
)

// subscriber owns a delivery channel and the pending queue that feeds it.
//
// Publish appends to the queue while holding the driver's read lock; a pump
// goroutine drains the queue into the channel while holding no lock at all.
// That split is the whole point: it lets the send into a slow subscriber's
// channel block without stalling the bus, because the goroutine that blocks
// holds nothing that Subscribe or Unsubscribe needs.
//
// The pump also owns close(ch). Closing from anywhere else would race the
// pump's send.
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

// offer appends msg to the pending queue. It never blocks, so it is safe to
// call while holding the driver's read lock.
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

// claimEviction reports whether this call is the one responsible for evicting
// the subscriber. Concurrent publishes can all observe a full queue.
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

// stop tears the subscription down and is safe to call more than once: a
// subscriber that was evicted for lagging may still be explicitly
// unsubscribed, and Stop may run after either.
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

// wait blocks until the pump has exited and ch is closed.
func (s *subscriber) wait() { <-s.done }

// shouldLogDrop throttles the drop warning. A subscriber that is wedged under
// sustained load would otherwise turn every dropped message into a log line.
func shouldLogDrop(n uint64) bool {
	return n == 1 || n%1024 == 0
}

// laggard is a subscriber that filled its queue, paired with the topic it
// subscribed to. That is not necessarily the topic the message was published
// to: a subscriber on "app" receives messages published to "app/module", and
// evicting it means removing it from "app".
type laggard struct {
	sub   *subscriber
	topic string
}

// dispatcher holds the delivery policy and counters shared by every driver.
type dispatcher struct {
	log  log.Logger
	opts *options

	dropped atomic.Uint64
	evicted atomic.Uint64
}

func newDispatcher(logger log.Logger, opts ...Option) *dispatcher {
	return &dispatcher{log: logger, opts: newOptions(opts...)}
}

// Dropped returns the number of messages discarded because a subscriber could
// not keep up.
func (d *dispatcher) Dropped() uint64 { return d.dropped.Load() }

// Evicted returns the number of subscribers removed because they could not
// keep up.
func (d *dispatcher) Evicted() uint64 { return d.evicted.Load() }

// offer hands msg to sub and reports whether sub must now be evicted. It never
// blocks, so it is safe under the driver's read lock. Eviction itself is not:
// it needs the write lock, and Go's RWMutex is not upgradable.
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

// fanout offers msg to every local subscriber whose subscription topic matches,
// skipping the publisher itself, and returns the laggards for the caller to
// evict once it has released the read lock. Callers must hold the read lock.
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

// claim reports whether this caller owns evicting l, logging the eviction when
// it does. Concurrent publishes can all observe the same full queue.
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
	Publisher string          `json:"publisher"`
	Topic     string          `json:"topic"`
	Kind      string          `json:"kind"`
	Payload   json.RawMessage `json:"payload"`
}

// topicMatches checks if a subscription topic matches a publish topic.
// "app" matches "app", "app/module", "app/module/component".
func topicMatches(subTopic, eventTopic string) bool {
	if subTopic == eventTopic {
		return true
	}
	return strings.HasPrefix(eventTopic, subTopic+"/")
}
