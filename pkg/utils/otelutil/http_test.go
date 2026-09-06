package otelutil_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"go.opentelemetry.io/otel/semconv/v1.43.0/httpconv"

	"github.com/xhanio/framingo/pkg/utils/otelutil"
)

// The convention's set maps onto itself; anything else - a made-up method or
// a known one in the wrong case, which net/http lets through - is _OTHER,
// so no client can mint series.
func TestRequestMethod(t *testing.T) {
	assert.Equal(t, httpconv.RequestMethodGet, otelutil.RequestMethod("GET"))
	assert.Equal(t, httpconv.RequestMethodQuery, otelutil.RequestMethod("QUERY"))
	assert.Equal(t, httpconv.RequestMethodOther, otelutil.RequestMethod("get"))
	assert.Equal(t, httpconv.RequestMethodOther, otelutil.RequestMethod("FOO"))
	assert.Equal(t, httpconv.RequestMethodOther, otelutil.RequestMethod(""))
}
