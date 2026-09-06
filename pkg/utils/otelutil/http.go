package otelutil

import (
	"net/http"

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
