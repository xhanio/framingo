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

// shutdownTimeout keeps a stalled exporter from wedging a restart or stop.
const shutdownTimeout = 5 * time.Second

// controller owns the swappable pipeline: provider is never nil (noop when
// idle), sdk is the same object and nil when idle.
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

func (c *controller) current() trace.TracerProvider {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.provider
}

// retire degrades to noop and returns the old provider for the caller to flush;
// stopped is set here so Alive stays truthful during a slow flush.
func (c *controller) retire(stopped bool) *sdktrace.TracerProvider {
	c.mu.Lock()
	defer c.mu.Unlock()
	sdk := c.sdk
	c.sdk = nil
	c.provider = noop.NewTracerProvider()
	c.stopped = stopped
	return sdk
}

func (c *controller) promote(tp *sdktrace.TracerProvider) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.sdk = tp
	c.provider = tp
}

// stopped means the absence of a provider is deliberate.
func (c *controller) state() (live bool, stopped bool) {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.sdk != nil, c.stopped
}

func (c *controller) flush(ctx context.Context, retired *sdktrace.TracerProvider) error {
	if retired == nil {
		return nil
	}
	ctx, cancel := context.WithTimeout(ctx, shutdownTimeout)
	defer cancel()
	return errors.Wrap(retired.Shutdown(ctx))
}
