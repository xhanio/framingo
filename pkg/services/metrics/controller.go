package metrics

import (
	"context"
	"sync"
	"time"

	"github.com/xhanio/errors"
	sdkmetric "go.opentelemetry.io/otel/sdk/metric"
)

// shutdownTimeout keeps a stalled collector from wedging a restart or stop.
const shutdownTimeout = 5 * time.Second

// exportTimeout matches the SDK PeriodicReader's default.
const exportTimeout = 30 * time.Second

// controller owns the swappable exporter (nil while disabled or failed).
type controller struct {
	mu       sync.RWMutex
	exporter sdkmetric.Exporter
	stopped  bool

	lastExport time.Time
	lastErr    error
}

func newController() *controller {
	return &controller{}
}

func (c *controller) current() sdkmetric.Exporter {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.exporter
}

// retire detaches the exporter for the caller to shut down; an in-flight
// export may still fail against it, losing one window.
func (c *controller) retire(stopped bool) sdkmetric.Exporter {
	c.mu.Lock()
	defer c.mu.Unlock()
	exp := c.exporter
	c.exporter = nil
	c.stopped = stopped
	return exp
}

// swap is one critical section so a probe never sees no exporter mid-restart.
func (c *controller) swap(exp sdkmetric.Exporter) sdkmetric.Exporter {
	c.mu.Lock()
	defer c.mu.Unlock()
	old := c.exporter
	c.exporter = exp
	c.stopped = false
	return old
}

// stopped means the absence of an exporter is deliberate.
func (c *controller) state() (live bool, stopped bool) {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.exporter != nil, c.stopped
}

// export failures surface only in Info: failing Alive would flap the service.
func (c *controller) recordExport(err error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.lastExport = time.Now()
	c.lastErr = err
}

func (c *controller) lastOutcome() (time.Time, error) {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.lastExport, c.lastErr
}

func (c *controller) shutdown(ctx context.Context, retired sdkmetric.Exporter) error {
	if retired == nil {
		return nil
	}
	ctx, cancel := context.WithTimeout(ctx, shutdownTimeout)
	defer cancel()
	return errors.Wrap(retired.Shutdown(ctx))
}
