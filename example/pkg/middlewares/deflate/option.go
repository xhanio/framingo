package deflate

type Option func(*middleware)

// WithMaxDecompressed overrides DefaultMaxDecompressed, the ceiling on how
// large a single request body may expand to after inflation.
func WithMaxDecompressed(n int) Option {
	return func(m *middleware) {
		if n > 0 {
			m.maxDecompressed = n
		}
	}
}
