package server

import (
	"net/http"

	"github.com/xhanio/framingo/pkg/types/api"
)

var httpMethods = map[string]bool{
	http.MethodGet:     true,
	http.MethodHead:    true,
	http.MethodPost:    true,
	http.MethodPut:     true,
	http.MethodPatch:   true,
	http.MethodDelete:  true,
	http.MethodOptions: true,
	http.MethodTrace:   true,
	http.MethodConnect: true,
	api.MethodAny:      true,
}

func validHTTPMethod(method string) bool {
	return httpMethods[method]
}
