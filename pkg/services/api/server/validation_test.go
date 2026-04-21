package server

import (
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/xhanio/framingo/pkg/types/api"
)

func TestValidHTTPMethod(t *testing.T) {
	valid := []string{
		http.MethodGet, http.MethodHead, http.MethodPost, http.MethodPut,
		http.MethodPatch, http.MethodDelete, http.MethodOptions,
		http.MethodTrace, http.MethodConnect, api.MethodAny,
	}
	for _, m := range valid {
		assert.True(t, validHTTPMethod(m), "expected %s to be valid", m)
	}

	invalid := []string{"", "INVALID", "any", "get", "PROPFIND"}
	for _, m := range invalid {
		assert.False(t, validHTTPMethod(m), "expected %s to be invalid", m)
	}
}
