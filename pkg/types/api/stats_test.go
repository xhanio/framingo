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
		expected HandlerKey
	}{
		{
			name:     "simple path",
			req:      RequestInfo{Server: "http", Method: "GET", RawPath: "/api/users"},
			prefix:   "/",
			expected: HandlerKey{Server: "http", Method: "GET", Path: "/api/users"},
		},
		{
			name:     "with endpoint prefix",
			req:      RequestInfo{Server: "http", Method: "POST", RawPath: "/v1/api/users"},
			prefix:   "/v1",
			expected: HandlerKey{Server: "http", Method: "POST", Path: "/api/users"},
		},
		{
			name:     "wildcard pattern",
			req:      RequestInfo{Server: "http", Method: "GET", RawPath: "/api/*"},
			prefix:   "/",
			expected: HandlerKey{Server: "http", Method: "GET", Path: "/api/*"},
		},
		{
			name:     "wildcard with endpoint prefix",
			req:      RequestInfo{Server: "http", Method: "POST", RawPath: "/v1/proxy/*"},
			prefix:   "/v1",
			expected: HandlerKey{Server: "http", Method: "POST", Path: "/proxy/*"},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.expected, tt.req.Key(tt.prefix))
		})
	}
}
