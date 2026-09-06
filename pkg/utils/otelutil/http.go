package otelutil

import (
	"net/http"
	"strconv"

	"go.opentelemetry.io/otel/semconv/v1.43.0/httpconv"
)

// RequestMethod maps a request method onto the http.request.method
// convention's value set; anything outside it is _OTHER - the convention's
// guard against a client minting a new series, or span attribute value, per
// made-up or oddly-cased method. net/http validates a method's characters
// but keeps its spelling, so "get" and "FOO" both arrive verbatim. Shared by
// the api server's metrics and the trace middleware's span, so both agree.
func RequestMethod(method string) httpconv.RequestMethodAttr {
	switch method {
	case http.MethodConnect:
		return httpconv.RequestMethodConnect
	case http.MethodDelete:
		return httpconv.RequestMethodDelete
	case http.MethodGet:
		return httpconv.RequestMethodGet
	case http.MethodHead:
		return httpconv.RequestMethodHead
	case http.MethodOptions:
		return httpconv.RequestMethodOptions
	case http.MethodPatch:
		return httpconv.RequestMethodPatch
	case http.MethodPost:
		return httpconv.RequestMethodPost
	case http.MethodPut:
		return httpconv.RequestMethodPut
	case http.MethodTrace:
		return httpconv.RequestMethodTrace
	case "QUERY":
		return httpconv.RequestMethodQuery
	}
	return httpconv.RequestMethodOther
}

// ProtocolVersion is the network.protocol.version value for a request: "1.0"
// or "1.1" for HTTP/1, and the bare major - "2", "3" - above it, the
// convention's spelling rather than Go's "HTTP/2.0". Shared by the api
// server's metrics and the trace middleware's span, like RequestMethod.
func ProtocolVersion(r *http.Request) string {
	if r.ProtoMajor == 1 {
		return "1." + strconv.Itoa(r.ProtoMinor)
	}
	return strconv.Itoa(r.ProtoMajor)
}
