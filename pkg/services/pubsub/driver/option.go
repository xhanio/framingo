package driver

type OnFull int

const (
	// DropMessage is silent: the subscriber never learns it missed one.
	DropMessage OnFull = iota
	// DropSubscriber closes the channel, so a subscriber resuming from its own cursor loses nothing.
	DropSubscriber
)

// a ceiling, not a preallocation: reaching it means the subscriber stopped draining
const defaultQueueCap = 50000

type Stats interface {
	Dropped() uint64
	Evicted() uint64
}

type options struct {
	onFull   OnFull
	queueCap int
	chanBuf  int
}

func newOptions(opts ...Option) *options {
	o := &options{}
	o.applyOptions(opts...)
	o.setDefaults()
	return o
}

func (o *options) applyOptions(opts ...Option) {
	for _, opt := range opts {
		opt(o)
	}
}

// onFull's zero value DropMessage is the default
func (o *options) setDefaults() {
	if o.queueCap == 0 {
		o.queueCap = defaultQueueCap
	}
	if o.chanBuf == 0 {
		o.chanBuf = channelBufferSize
	}
}

type Option func(*options)

func WithOnFull(v OnFull) Option {
	return func(o *options) { o.onFull = v }
}

func WithQueueCap(n int) Option {
	return func(o *options) {
		o.queueCap = n
	}
}

func WithChannelBuffer(n int) Option {
	return func(o *options) {
		o.chanBuf = n
	}
}
