package printutil

type Option func(*table)

func (t *table) apply(opts ...Option) {
	for _, opt := range opts {
		opt(t)
	}
}

func FullDisplay() Option {
	return func(t *table) {
		t.full = true
	}
}
