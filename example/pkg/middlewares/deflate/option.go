package deflate

import "github.com/xhanio/framingo/pkg/utils/nameutil"

type Option func(*middleware)

func (m *middleware) applyOptions(opts ...Option) {
	for _, opt := range opts {
		opt(m)
	}
}

func (m *middleware) setDefaults() {
	if m.name == "" {
		m.name = nameutil.Package(m)
	}
	if m.maxDecompressed == 0 {
		m.maxDecompressed = DefaultMaxDecompressed
	}
}

// WithName also renames the config key, so two instances can run side by side.
func WithName(name string) Option {
	return func(m *middleware) {
		m.name = name
	}
}

func WithMaxDecompressed(n int) Option {
	return func(m *middleware) {
		m.maxDecompressed = n
	}
}
