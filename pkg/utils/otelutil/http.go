package otelutil

import (
	"net/http"
	"strconv"

	"go.opentelemetry.io/otel/semconv/v1.43.0/httpconv"
)

// RequestMethod is case-sensitive: anything outside the known set (including "get") is _OTHER,
// so clients cannot mint new series.
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

// ProtocolVersion spells semconv's "1.1", "2", "3", not Go's "HTTP/2.0".
func ProtocolVersion(r *http.Request) string {
	if r.ProtoMajor == 1 {
		return "1." + strconv.Itoa(r.ProtoMinor)
	}
	return strconv.Itoa(r.ProtoMajor)
}
