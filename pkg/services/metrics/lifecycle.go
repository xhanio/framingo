package metrics

import (
	"context"
	"io"
	"time"

	"github.com/xhanio/errors"
	sdkmetric "go.opentelemetry.io/otel/sdk/metric"
	"go.opentelemetry.io/otel/sdk/metric/metricdata"
	"gopkg.in/yaml.v3"

	"github.com/xhanio/framingo/pkg/types/info"
	"github.com/xhanio/framingo/pkg/utils/confutil"
	"github.com/xhanio/framingo/pkg/utils/printutil"
)

func (m *manager) Init(ctx context.Context) error {
	config := confutil.FromContext(ctx)
	// Dynamic config reloads through the very options the wiring layer used,
	// so a value has one way in and setDefaults has the only say on what a
	// zero falls back to - the constructor's order, repeated. Config is the
	// authority here: an absent key reads zero and lands on the default
	// rather than preserving whatever construction happened to set. Under
	// the lock because probes, Info and the export loop read these fields
	// concurrently. The exporter block stays opaque - the chosen factory
	// strict-decodes its own struct.
	m.mu.Lock()
	m.applyOptions(
		WithEnabled(config.GetBool("metrics.enabled")),
		WithExporter(
			config.GetString("metrics.exporter"),
			config.GetStringMap("metrics.config"),
		),
		WithInterval(config.GetDuration("metrics.interval")),
	)
	m.setDefaults()
	enabled := m.enabled
	m.mu.Unlock()
	// Init runs on first startup and on every restart. The provider and
	// reader stay - cached instruments must survive - so unlike tracing
	// there is nothing to flush here: whatever aggregated meanwhile is
	// picked up by the next export cycle through the same reader. On the
	// happy path the replacement is built first and swapped in atomically,
	// so a probe landing mid-restart still sees a live exporter; failure
	// still retires the previous one - enabled with no exporter is the
	// wiring fault Alive reports, and config is the authority a running
	// state must not outlive. A failed shutdown only loses that exporter's
	// buffered data - never the restart.
	if !enabled {
		if err := m.controller.shutdown(m.controller.retire(false)); err != nil {
			m.log.Warnf("failed to shut down retiring metrics exporter: %s", err)
		}
		return nil
	}
	exp, err := m.buildExporter(ctx)
	if err != nil {
		if serr := m.controller.shutdown(m.controller.retire(false)); serr != nil {
			m.log.Warnf("failed to shut down retiring metrics exporter: %s", serr)
		}
		return err
	}
	// the identity check only matters for the injected test seam, where
	// buildExporter hands back the same instance every Init - a registry
	// factory always constructs a fresh one
	if old := m.controller.swap(exp); old != exp {
		if serr := m.controller.shutdown(old); serr != nil {
			m.log.Warnf("failed to shut down retiring metrics exporter: %s", serr)
		}
	}
	return nil
}

// buildExporter resolves where metrics go: the injected instance when the
// test seam is set, else the registry factory named by exporterName fed the
// marshaled config block. It reads the config fields unlocked - Init, the
// only writer, already rewrote them earlier in the same call, and
// supervisor ops serialize.
func (m *manager) buildExporter(ctx context.Context) (sdkmetric.Exporter, error) {
	if m.exporter != nil {
		return m.exporter, nil
	}
	if m.exporterName == "" {
		return nil, errors.Newf("metrics %s is enabled but no exporter is configured", m.Name())
	}
	f, err := lookupFactory(m.exporterName)
	if err != nil {
		return nil, errors.Wrap(err)
	}
	var raw []byte
	if len(m.exporterConfig) > 0 {
		if raw, err = yaml.Marshal(m.exporterConfig); err != nil {
			return nil, errors.Wrapf(err, "invalid metrics exporter config")
		}
	}
	// otlp exporters connect lazily, so an unreachable collector does not
	// fail the build - matching the probe doctrine that a collector outage
	// must never take the app down
	exp, err := f(ctx, raw)
	if err != nil {
		return nil, errors.Wrapf(err, "failed to build %s metric exporter", m.exporterName)
	}
	return exp, nil
}

// export runs one collect+export cycle against the current exporter - the
// primitive both the Start loop and the final Stop flush drive. A nil
// exporter (disabled, or a failed Init) is a quiet no-op: instruments keep
// aggregating into the SDK and the next enabled cycle picks the state up.
func (m *manager) export(ctx context.Context) error {
	exp := m.controller.current()
	if exp == nil {
		return nil
	}
	ctx, cancel := context.WithTimeout(ctx, exportTimeout)
	defer cancel()
	var rm metricdata.ResourceMetrics
	if err := m.reader.Collect(ctx, &rm); err != nil {
		m.controller.recordExport(err)
		return errors.Wrap(err)
	}
	// nothing recorded yet: skip the empty envelope so a stdout exporter
	// stays quiet instead of printing one per interval (cumulative otlp
	// needs no empty pushes either). Still record the healthy outcome, so
	// Info can tell an idle loop from one that never ran.
	if len(rm.ScopeMetrics) == 0 {
		m.controller.recordExport(nil)
		return nil
	}
	err := exp.Export(ctx, &rm)
	m.controller.recordExport(err)
	return errors.Wrap(err)
}

// Start runs the export loop - unlike tracing's no-op Start, Daemon is
// genuinely meaningful here: the ManualReader only collects when this loop
// (or the final flush in Stop) drives it.
func (m *manager) Start(_ context.Context) error {
	if m.loopCancel != nil {
		m.log.Warnf("%s already started", m.Name())
		return nil
	}
	// Detached from the caller's context on purpose: the loop's lifetime is
	// Stop's to end, and Stop must flush before the exporter goes away.
	ctx, cancel := context.WithCancel(context.Background())
	m.loopCancel = cancel
	m.wg.Add(1)
	go m.run(ctx)
	return nil
}

// run ticks at the configured interval, re-reading it each cycle so an Init
// that changed metrics.interval takes effect at the next tick without
// restarting the loop.
func (m *manager) run(ctx context.Context) {
	defer m.wg.Done()
	interval := m.currentInterval()
	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			if cur := m.currentInterval(); cur != interval {
				interval = cur
				ticker.Reset(interval)
			}
			if err := m.export(ctx); err != nil {
				m.log.Warnf("metrics export failed: %s", err)
			}
		}
	}
}

func (m *manager) currentInterval() time.Duration {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.interval
}

// Stop ends the loop, performs the final collect+export (the flush), and
// shuts the exporter down. The flush is best-effort - a dead collector
// loses the last window's data, never the shutdown - while a failed
// exporter shutdown is returned. Idempotent, like tracing's Stop.
func (m *manager) Stop(_ bool) error {
	if m.loopCancel != nil {
		m.loopCancel()
		m.loopCancel = nil
		m.wg.Wait()
	}
	// flush before retiring: the exporter must still be installed for the
	// final cycle to reach it. Bounded by shutdownTimeout, not the looser
	// exportTimeout - a stalled collector must not wedge the supervisor's
	// stop sequence (export's own timeout nests inside this one).
	ctx, cancel := context.WithTimeout(context.Background(), shutdownTimeout)
	defer cancel()
	if err := m.export(ctx); err != nil {
		m.log.Warnf("final metrics flush failed: %s", err)
	}
	return m.controller.shutdown(m.controller.retire(true))
}

func (m *manager) Info(w io.Writer, debug bool) {
	m.mu.RLock()
	enabled := m.enabled
	exporter := m.exporterName
	if m.exporter != nil {
		exporter = "custom"
	}
	interval := m.interval
	m.mu.RUnlock()
	state := "idle"
	if live, _ := m.controller.state(); live {
		state = "exporting"
	}
	t := printutil.NewTable(w)
	t.Header(m.Name())
	t.Title("stat", "value")
	t.Row("enabled", enabled)
	t.Row("exporter", exporter)
	t.Row("state", state)
	if debug {
		t.Row("interval", interval)
		last, lastErr := m.controller.lastOutcome()
		if !last.IsZero() {
			t.Row("last_export", last.Format(time.RFC3339))
		}
		if lastErr != nil {
			t.Row("last_error", lastErr)
		}
		t.Row("service_name", info.ApplicationName)
	}
	t.NewLine()
	t.Flush()
}
