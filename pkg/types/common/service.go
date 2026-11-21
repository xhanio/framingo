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

// Daemon may be started and stopped repeatedly on one instance. What a start runs
// lives until Stop: Start's ctx bounds the start alone and is never kept, its values
// aside. Stop refuses new work, cancels and awaits the rest (an earlier Stop's
// leftovers included) until ctx ends, then forces what it can and returns an error
// wrapping ctx's; it returns only after intake has stopped, and leftovers must not
// reach a later Start.
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

// DependencyState is what a dependency has become, as the supervisor tells a
// dependent that runs on beside it.
type DependencyState int

const (
	// DependencyRunning: the dependency runs again, and has forgotten what was
	// registered with it before it stopped.
	DependencyRunning DependencyState = iota + 1
	// DependencyStopped: the dependency has stopped, by hand or for a restart,
	// and has forgotten what was registered with it.
	DependencyStopped
	// DependencyFailed: the dependency's Init or Start failed; it is down until
	// a restart succeeds, which DependencyRunning then says.
	DependencyFailed
)

func (s DependencyState) String() string {
	switch s {
	case DependencyRunning:
		return "running"
	case DependencyStopped:
		return "stopped"
	case DependencyFailed:
		return "failed"
	}
	return "unknown"
}

// Reconcilable is a service that registers something with a dependency, a
// subscription or a task, which the dependency forgets when it stops. When the
// supervisor stops or starts that dependency alone, it calls Reconcile on each
// such dependent that keeps running, in dependency order, under its lock:
// Reconcile brings what dep holds for this service in line with state, putting
// back on DependencyRunning what this service holds, and nothing else. It
// relies on no state but the one it is given: a call that finds nothing to do
// changes nothing, and a state it does not act on returns nil. It runs beside
// the service's own work, bounded by ctx, so it registers and never does slow
// work, nor calls the supervisor. An error fails the service, which the
// supervisor then restarts, its own Init and Start registering everything.
type Reconcilable interface {
	Reconcile(ctx context.Context, dep Service, state DependencyState) error
}
