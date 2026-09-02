package metrics

import (
	"context"
	"sync"
	"time"

	"go.opentelemetry.io/otel/attribute"
	sdkmetric "go.opentelemetry.io/otel/sdk/metric"
	"go.opentelemetry.io/otel/sdk/resource"
	semconv "go.opentelemetry.io/otel/semconv/v1.43.0"

	"github.com/xhanio/framingo/pkg/types/common"
	"github.com/xhanio/framingo/pkg/types/info"
	"github.com/xhanio/framingo/pkg/utils/log"
)

type manager struct {
	name string
	log  log.Logger

	enabled        bool
	exporterName   string
	exporterConfig map[string]any
	interval       time.Duration

	// exporter is an injected instance that bypasses the registry - the
	// package tests' seam, assigned directly on the concrete manager. nil
	// means Init builds one from exporterName. Production custom exporters
	// go through Register instead: an app-owned Factory captures any
	// construction.
	exporter sdkmetric.Exporter

	// provider and reader are built once at construction and never replaced
	// - the identity model.Meter promises. Restarts swap only the exporter
	// behind them (the controller's job), so instruments cached by callers
	// stay valid for the life of the process. Both are safe to read without
	// the lock: never written after construction.
	provider *sdkmetric.MeterProvider
	reader   *sdkmetric.ManualReader

	// mu guards the dynamic config fields above (enabled, exporterName,
	// exporterConfig, interval): Init rewrites them on every restart while
	// Alive, Info and the export loop keep reading them. The exporter slot
	// has its own lock inside controller.
	mu         sync.RWMutex
	controller *controller

	// export loop lifecycle. Start and Stop are serialized by the
	// supervisor's op lock, so these need no guard of their own.
	loopCancel context.CancelFunc
	wg         sync.WaitGroup
}

var _ Manager = (*manager)(nil)

func New(opts ...Option) Manager {
	return newManager(opts...)
}

// newManager returns the concrete manager, the form package tests construct.
// The provider pipeline is built here, once: unlike tracing, the provider
// must survive every restart because callers cache instruments created from
// it - Init only ever swaps the exporter behind the shared reader.
func newManager(opts ...Option) *manager {
	m := &manager{
		controller: newController(),
		reader:     sdkmetric.NewManualReader(),
	}
	m.applyOptions(opts...)
	m.setDefaults()
	m.log = m.log.By(m)
	// The metric resource is the build's application identity, nothing else
	// - same rules as the span resource: a build without ldflags injection
	// omits the attribute rather than setting it blank, so a backend shows
	// its own unknown-service placeholder instead of a nameless service.
	var attrs []attribute.KeyValue
	if info.ApplicationName != "" {
		attrs = append(attrs, semconv.ServiceName(info.ApplicationName))
	}
	if info.ApplicationVersion != "" {
		attrs = append(attrs, semconv.ServiceVersion(info.ApplicationVersion))
	}
	m.provider = sdkmetric.NewMeterProvider(
		sdkmetric.WithResource(resource.NewWithAttributes(semconv.SchemaURL, attrs...)),
		sdkmetric.WithReader(m.reader),
	)
	return m
}

func (m *manager) Name() string {
	return m.name
}

func (m *manager) Dependencies() []common.Service {
	return nil
}
