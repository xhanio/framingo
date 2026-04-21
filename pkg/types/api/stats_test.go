package api

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestRequestInfoKey(t *testing.T) {
	tests := []struct {
		name     string
		req      RequestInfo
		prefix   string
		expected string
	}{
		{
			name:     "simple path",
			req:      RequestInfo{Server: "http", Method: "GET", RawPath: "/api/users"},
			prefix:   "/",
			expected: "http<GET>/api/users",
		},
		{
			name:     "with endpoint prefix",
			req:      RequestInfo{Server: "http", Method: "POST", RawPath: "/v1/api/users"},
			prefix:   "/v1",
			expected: "http<POST>/api/users",
		},
		{
			name:     "wildcard pattern",
			req:      RequestInfo{Server: "http", Method: "GET", RawPath: "/api/*"},
			prefix:   "/",
			expected: "http<GET>/api/*",
		},
		{
			name:     "wildcard with endpoint prefix",
			req:      RequestInfo{Server: "http", Method: "POST", RawPath: "/v1/proxy/*"},
			prefix:   "/v1",
			expected: "http<POST>/proxy/*",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.expected, tt.req.Key(tt.prefix))
		})
	}
}
