package driver

// OnFull selects what a driver does when a subscriber's pending queue is full,
// meaning the subscriber is not draining its channel fast enough.
type OnFull int

const (
	// DropMessage discards the message and keeps the subscription. The
	// subscriber never learns that it missed a message.
	DropMessage OnFull = iota
	// DropSubscriber closes the subscriber's channel and removes it from the
	// topic. A subscriber that reconnects and resumes from its own cursor
	// loses nothing; one that is silently skipped loses a message forever.
	DropSubscriber
)

// defaultQueueCap bounds a subscriber's pending queue. The queue grows on
// demand, so this is a ceiling rather than a preallocation: reaching it means
// the subscriber has stopped draining entirely, not that it is briefly slow.
const defaultQueueCap = 50000

// Stats is implemented by drivers that report delivery statistics.
type Stats interface {
	// Dropped returns the number of messages discarded because a subscriber
	// could not keep up.
	Dropped() uint64
	// Evicted returns the number of subscribers removed because they could
	// not keep up.
	Evicted() uint64
}

type options struct {
	onFull   OnFull
	queueCap int
	chanBuf  int
}

func newOptions(opts ...Option) *options {
	o := &options{
		onFull:   DropMessage,
		queueCap: defaultQueueCap,
		chanBuf:  channelBufferSize,
	}
	for _, opt := range opts {
		opt(o)
	}
	return o
}

// Option configures a driver.
type Option func(*options)

// WithOnFull sets the policy applied when a subscriber's queue is full.
// The default is DropMessage, which preserves the historical behavior.
func WithOnFull(v OnFull) Option {
	return func(o *options) { o.onFull = v }
}

// WithQueueCap bounds each subscriber's pending queue.
func WithQueueCap(n int) Option {
	return func(o *options) {
		if n > 0 {
			o.queueCap = n
		}
	}
}

// WithChannelBuffer sets the buffer size of the channel handed to subscribers.
func WithChannelBuffer(n int) Option {
	return func(o *options) {
		if n > 0 {
			o.chanBuf = n
		}
	}
}
