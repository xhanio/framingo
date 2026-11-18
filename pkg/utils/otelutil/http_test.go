package otelutil

import (
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"
	"go.opentelemetry.io/otel/semconv/v1.43.0/httpconv"
)

func TestRequestMethod(t *testing.T) {
	assert.Equal(t, httpconv.RequestMethodGet, RequestMethod("GET"))
	assert.Equal(t, httpconv.RequestMethodQuery, RequestMethod("QUERY"))
	assert.Equal(t, httpconv.RequestMethodOther, RequestMethod("get"))
	assert.Equal(t, httpconv.RequestMethodOther, RequestMethod("FOO"))
	assert.Equal(t, httpconv.RequestMethodOther, RequestMethod(""))
}

func TestProtocolVersion(t *testing.T) {
	assert.Equal(t, "1.1", ProtocolVersion(&http.Request{ProtoMajor: 1, ProtoMinor: 1}))
	assert.Equal(t, "1.0", ProtocolVersion(&http.Request{ProtoMajor: 1, ProtoMinor: 0}))
	assert.Equal(t, "2", ProtocolVersion(&http.Request{ProtoMajor: 2, ProtoMinor: 0}))
	assert.Equal(t, "3", ProtocolVersion(&http.Request{ProtoMajor: 3}))
}
