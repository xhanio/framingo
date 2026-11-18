package common

import (
	"context"
	"io"
)

type Service interface {
	Named
	// Dependencies are initialized and started first; nil entries are skipped.
	Dependencies() []Service
}

// Daemon may be started and stopped repeatedly on one instance. Stop refuses new
// work, cancels and awaits the rest (an earlier Stop's leftovers included) until
// ctx ends, then forces what it can and returns an error wrapping ctx's; it
// returns only after intake has stopped, and leftovers must not reach a later Start.
type Daemon interface {
	Start(ctx context.Context) error
	Stop(ctx context.Context) error
}

// Init runs on first startup and every restart: load dynamic config here.
type Initializable interface {
	Init(ctx context.Context) error
}

type Debuggable interface {
	Info(w io.Writer, debug bool)
}

// Probes honour the caller's ctx deadline; an implementation may only tighten it.
type Liveness interface {
	Alive(ctx context.Context) error
}

type Readiness interface {
	Ready(ctx context.Context) error
}
