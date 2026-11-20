package metrics

import (
	"sync"
	"time"

	"go.opentelemetry.io/otel/attribute"
	sdkmetric "go.opentelemetry.io/otel/sdk/metric"
	"go.opentelemetry.io/otel/sdk/resource"
	semconv "go.opentelemetry.io/otel/semconv/v1.43.0"

	"github.com/xhanio/framingo/pkg/types/common"
	"github.com/xhanio/framingo/pkg/types/info"
	"github.com/xhanio/framingo/pkg/utils/log"
	"github.com/xhanio/framingo/pkg/utils/syncutil"
)

type manager struct {
	name string
	log  log.Logger

	enabled        bool
	exporterName   string
	exporterConfig map[string]any
	interval       time.Duration

	// exporter is the tests' seam bypassing the registry; nil means Init builds one.
	exporter sdkmetric.Exporter

	// never replaced after construction (model.Meter's promise), so read unlocked
	provider *sdkmetric.MeterProvider
	reader   *sdkmetric.ManualReader

	// mu guards enabled, exporterName, exporterConfig and interval.
	mu         sync.RWMutex
	controller *controller

	loop syncutil.Loop // the export loop: Stop ends it, then flushes
}

var _ Manager = (*manager)(nil)

func New(opts ...Option) Manager {
	return newManager(opts...)
}

func newManager(opts ...Option) *manager {
	m := &manager{
		controller: newController(),
		reader:     sdkmetric.NewManualReader(),
	}
	m.applyOptions(opts...)
	m.setDefaults()
	m.log = m.log.By(m)
	// omit unset attributes rather than blank them, so backends show their placeholder
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
