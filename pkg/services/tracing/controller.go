package tracing

import (
	"context"
	"sync"
	"time"

	"github.com/xhanio/errors"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	"go.opentelemetry.io/otel/trace"
	"go.opentelemetry.io/otel/trace/noop"
)

// shutdownTimeout bounds the final flush of a retiring provider so a stalled
// exporter cannot wedge a restart or the supervisor's stop sequence. A
// deliberate constant, not an option: it is a lifecycle invariant sized to
// the batcher defaults, and any real budget must come from the outer stop
// policy, not a per-service knob.
const shutdownTimeout = 5 * time.Second

// controller owns the live provider pipeline and the discipline around
// swapping it: the API-typed handle served to callers (never nil - noop when
// no pipeline is live), the same object at its concrete SDK type (the only
// one with Shutdown, whose nil-ness doubles as the state bit), and the
// deliberate-stop marker. All access goes through its methods, so the lock
// lets requests keep resolving the provider while Init and Stop swap it.
type controller struct {
	mu       sync.RWMutex
	provider trace.TracerProvider
	sdk      *sdktrace.TracerProvider
	stopped  bool
}

func newController() *controller {
	return &controller{
		provider: noop.NewTracerProvider(),
	}
}

// current returns the handle callers trace with - the live SDK provider, or
// noop when none is live.
func (c *controller) current() trace.TracerProvider {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.provider
}

// retire swaps the pipeline out - the handle degrades to noop so no new span
// reaches a closing pipeline, and stopped records whether this is a
// deliberate stop - then returns the retired provider for the caller to
// flush. Recording stopped here, not after the flush, keeps Alive truthful
// while a slow flush is still draining.
func (c *controller) retire(stopped bool) *sdktrace.TracerProvider {
	c.mu.Lock()
	defer c.mu.Unlock()
	sdk := c.sdk
	c.sdk = nil
	c.provider = noop.NewTracerProvider()
	c.stopped = stopped
	return sdk
}

// promote installs a freshly built pipeline as the live one.
func (c *controller) promote(tp *sdktrace.TracerProvider) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.sdk = tp
	c.provider = tp
}

// state reports the pipeline for probes and Info: live means a real provider
// is installed, stopped means the current absence of one is deliberate.
func (c *controller) state() (live bool, stopped bool) {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.sdk != nil, c.stopped
}

// flush drains whatever the retired pipeline's processor still buffers,
// bounded by shutdownTimeout.
func (c *controller) flush(retired *sdktrace.TracerProvider) error {
	if retired == nil {
		return nil
	}
	ctx, cancel := context.WithTimeout(context.Background(), shutdownTimeout)
	defer cancel()
	return errors.Wrap(retired.Shutdown(ctx))
}
