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
	"github.com/xhanio/framingo/pkg/utils/nameutil"
	"github.com/xhanio/framingo/pkg/utils/printutil"
)

func (m *manager) Init(ctx context.Context) error {
	config := confutil.FromContext(ctx)
	root := nameutil.ConfigRoot(m.name)
	// reload through the constructor's options so setDefaults decides every zero;
	// locked: probes, Info and the export loop read these fields
	m.mu.Lock()
	m.applyOptions(
		WithEnabled(config.GetBool("%s.enabled", root)),
		WithExporter(
			config.GetString("%s.exporter", root),
			config.GetStringMap("%s.config", root),
		),
		WithInterval(config.GetDuration("%s.interval", root)),
	)
	m.setDefaults()
	enabled := m.enabled
	m.mu.Unlock()
	// build before swapping so probes never see a gap; on failure still retire
	// the old exporter so Alive reports the fault
	if !enabled {
		if err := m.controller.shutdown(context.Background(), m.controller.retire(false)); err != nil {
			m.log.Warnf("failed to shut down retiring metrics exporter: %s", err)
		}
		return nil
	}
	exp, err := m.buildExporter(ctx)
	if err != nil {
		if serr := m.controller.shutdown(context.Background(), m.controller.retire(false)); serr != nil {
			m.log.Warnf("failed to shut down retiring metrics exporter: %s", serr)
		}
		return err
	}
	// the injected test exporter comes back identical on every Init
	if old := m.controller.swap(exp); old != exp {
		if serr := m.controller.shutdown(context.Background(), old); serr != nil {
			m.log.Warnf("failed to shut down retiring metrics exporter: %s", serr)
		}
	}
	return nil
}

// buildExporter reads config unlocked: Init is the only writer and calls it.
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
	// otlp connects lazily, so an unreachable collector does not fail the build
	exp, err := f(ctx, raw)
	if err != nil {
		return nil, errors.Wrapf(err, "failed to build %s metric exporter", m.exporterName)
	}
	return exp, nil
}

// export is a no-op without an exporter; the reader keeps aggregating meanwhile.
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
	// skip empty exports, but record the success so Info sees the loop ran
	if len(rm.ScopeMetrics) == 0 {
		m.controller.recordExport(nil)
		return nil
	}
	err := exp.Export(ctx, &rm)
	m.controller.recordExport(err)
	return errors.Wrap(err)
}

// Start drives the ManualReader, which never collects on its own.
func (m *manager) Start(_ context.Context) error {
	if m.loopCancel != nil {
		m.log.Warnf("%s already started", m.Name())
		return nil
	}
	// detached: only Stop ends the loop, after flushing
	ctx, cancel := context.WithCancel(context.Background())
	m.loopCancel = cancel
	m.wg.Add(1)
	go m.run(ctx)
	return nil
}

// run re-reads the interval each tick so Init can change it live.
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

// Stop is idempotent; the final flush is best-effort, only a failed exporter
// shutdown is returned.
func (m *manager) Stop(ctx context.Context) error {
	if m.loopCancel != nil {
		m.loopCancel()
		m.loopCancel = nil
		m.wg.Wait()
	}
	// flush before retiring, bounded by shutdownTimeout rather than exportTimeout
	flushCtx, cancel := context.WithTimeout(ctx, shutdownTimeout)
	defer cancel()
	if err := m.export(flushCtx); err != nil {
		m.log.Warnf("final metrics flush failed: %s", err)
	}
	return m.controller.shutdown(ctx, m.controller.retire(true))
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
