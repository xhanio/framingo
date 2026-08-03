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

// Liveness and Readiness probes take the caller's context: probes may do
// I/O (a database ping), and the caller owns the deadline budget and the
// shutdown signal. Implementations may layer their own tighter timeout on
// top, never a looser one.
type Liveness interface {
	Alive(ctx context.Context) error
}

type Readiness interface {
	Ready(ctx context.Context) error
}
