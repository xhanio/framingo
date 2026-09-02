package metrics

import (
	"github.com/xhanio/framingo/pkg/utils/otelutil"
)

var reg = otelutil.NewRegistry[Factory]("metrics exporter", "pkg/services/metrics/exporters/*")

func lookupFactory(name string) (Factory, error) {
	return reg.Lookup(name)
}
