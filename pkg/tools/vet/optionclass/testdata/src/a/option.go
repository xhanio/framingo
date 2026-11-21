package a

type manager struct {
	name     string
	interval int
}

type Option func(*manager)

func WithEarly(n int) Option { // want `option WithEarly is in no class`
	return func(m *manager) {}
}

//framingo:ignore optionclass a reason the test gives
func WithExcused(n int) Option {
	return func(m *manager) {}
}

// Construction options: fixed for the instance's life, set at New.

func WithName(name string) Option {
	return func(m *manager) { m.name = name }
}

// Reloadable options: each sets the default of a key Init re-reads.

// WithInterval sets the default of a.interval.
func WithInterval(n int) Option {
	return func(m *manager) { m.interval = n }
}

// unexported, a method, or no option: not this rule's
func withHidden() Option { return nil }

func (m *manager) WithMethod() {}
