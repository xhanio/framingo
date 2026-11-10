package common

import (
	"context"
	"io"
)

type Service interface {
	Named
	Dependencies() []Service
}

// Daemon is a service that runs. Start and Stop are called on one instance, and more than once.
//
// Stop ends what Start began: it takes no new work, cancels the work going on, and waits for all
// of it to end, what an earlier Stop left included, until ctx ends. If ctx ends first, it ends
// the work hard where it can and returns an error wrapping ctx's, leaving the rest to end on its
// own; an ended ctx makes it a forced stop. Either way it returns only once whatever takes new
// work has returned. Start works right after any Stop, and what a Stop left must not reach
// into it.
type Daemon interface {
	Start(ctx context.Context) error
	Stop(ctx context.Context) error
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
