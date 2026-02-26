package common

import (
	"context"
	"io"
)

type Service interface {
	Named
	Dependencies() []Service
}

type Daemon interface {
	Start(ctx context.Context) error
	Stop(wait bool) error
}

// Initializable is implemented by services that require initialization.
// Init is called on first startup and on every restart, making it the
// appropriate place to load dynamic configuration that may change between runs.
type Initializable interface {
	Init(ctx context.Context) error
}

type Debuggable interface {
	Info(w io.Writer, debug bool)
}
type Liveness interface {
	Alive() error
}

type Readiness interface {
	Ready() error
}
