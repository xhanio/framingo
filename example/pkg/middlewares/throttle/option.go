package throttle

type Option func(*middleware)

// WithLimit sets the instance's limit, applied per client IP on every route
// that attaches the middleware without a config of its own. A zero rps or
// burstSize means no limit, keeping the contract of the server option this
// middleware replaced.
func WithLimit(rps float64, burstSize int) Option {
	return func(m *middleware) {
		if rps == 0 || burstSize == 0 {
			// no throttle control
			return
		}
		m.rps = rps
		m.burst = burstSize
	}
}
