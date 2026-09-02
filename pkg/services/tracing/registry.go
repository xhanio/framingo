package tracing

import (
	"github.com/xhanio/framingo/pkg/utils/otelutil"
)

var reg = otelutil.NewRegistry[Factory]("tracing exporter", "pkg/services/tracing/exporters/*")

func lookupFactory(name string) (Factory, error) {
	return reg.Lookup(name)
}
