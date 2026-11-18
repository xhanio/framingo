package otelutil_test

import (
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"
	"go.opentelemetry.io/otel/semconv/v1.43.0/httpconv"

	"github.com/xhanio/framingo/pkg/utils/otelutil"
)

func TestRequestMethod(t *testing.T) {
	assert.Equal(t, httpconv.RequestMethodGet, otelutil.RequestMethod("GET"))
	assert.Equal(t, httpconv.RequestMethodQuery, otelutil.RequestMethod("QUERY"))
	assert.Equal(t, httpconv.RequestMethodOther, otelutil.RequestMethod("get"))
	assert.Equal(t, httpconv.RequestMethodOther, otelutil.RequestMethod("FOO"))
	assert.Equal(t, httpconv.RequestMethodOther, otelutil.RequestMethod(""))
}

func TestProtocolVersion(t *testing.T) {
	assert.Equal(t, "1.1", otelutil.ProtocolVersion(&http.Request{ProtoMajor: 1, ProtoMinor: 1}))
	assert.Equal(t, "1.0", otelutil.ProtocolVersion(&http.Request{ProtoMajor: 1, ProtoMinor: 0}))
	assert.Equal(t, "2", otelutil.ProtocolVersion(&http.Request{ProtoMajor: 2, ProtoMinor: 0}))
	assert.Equal(t, "3", otelutil.ProtocolVersion(&http.Request{ProtoMajor: 3}))
}
