package printutil

type Option func(*table)

func (t *table) applyOptions(opts ...Option) {
	for _, opt := range opts {
		opt(t)
	}
}

// Construction options: fixed for the instance's life, set at New (design/config.md).

func FullDisplay() Option {
	return func(t *table) {
		t.full = true
	}
}
