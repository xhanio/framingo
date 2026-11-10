package metrics

import (
	"context"
	"sync"
	"time"

	"github.com/xhanio/errors"
	sdkmetric "go.opentelemetry.io/otel/sdk/metric"
)

// shutdownTimeout bounds the shutdown of a retiring exporter so a stalled
// collector cannot wedge a restart or the supervisor's stop sequence. A
// deliberate constant, not an option: it is a lifecycle invariant, and any
// real budget must come from the outer stop policy, not a per-service knob.
const shutdownTimeout = 5 * time.Second

// exportTimeout bounds one collect+export cycle, matching the SDK
// PeriodicReader's default export timeout, so a hung collector delays the
// loop by at most one bounded cycle.
const exportTimeout = 30 * time.Second

// controller owns the exporter slot and the discipline around swapping it:
// the current exporter (nil while disabled or failed), the deliberate-stop
// marker, and the last export outcome for Info. The provider and reader
// live on the manager, fixed at construction - only the export path behind
// them ever changes.
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

// current returns the exporter the loop exports through - nil while none is
// installed.
func (c *controller) current() sdkmetric.Exporter {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.exporter
}

// retire swaps the exporter out - no new export cycle reaches a closing
// exporter, though a cycle already in flight may still error against it,
// losing at most that one window to the next cycle - and stopped records
// whether this is a deliberate stop, then returns the retired exporter for
// the caller to shut down.
func (c *controller) retire(stopped bool) sdkmetric.Exporter {
	c.mu.Lock()
	defer c.mu.Unlock()
	exp := c.exporter
	c.exporter = nil
	c.stopped = stopped
	return exp
}

// swap installs a freshly built exporter as the live one and hands back its
// predecessor in the same critical section, so a probe never observes an
// enabled manager exporterless mid-restart.
func (c *controller) swap(exp sdkmetric.Exporter) sdkmetric.Exporter {
	c.mu.Lock()
	defer c.mu.Unlock()
	old := c.exporter
	c.exporter = exp
	c.stopped = false
	return old
}

// state reports the export path for probes and Info: live means an exporter
// is installed, stopped means the current absence of one is deliberate.
func (c *controller) state() (live bool, stopped bool) {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.exporter != nil, c.stopped
}

// recordExport keeps the last outcome for Info. Export failures are
// deliberately invisible to Alive - the monitor restarts on liveness
// failure, and a dead collector must not flap the service - so Info is
// where they surface.
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

// shutdown closes a retired exporter, bounded by shutdownTimeout.
func (c *controller) shutdown(ctx context.Context, retired sdkmetric.Exporter) error {
	if retired == nil {
		return nil
	}
	ctx, cancel := context.WithTimeout(ctx, shutdownTimeout)
	defer cancel()
	return errors.Wrap(retired.Shutdown(ctx))
}
