package api

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestNewHandlerKey(t *testing.T) {
	tests := []struct {
		name     string
		group    *HandlerGroup
		handler  *Handler
		expected HandlerKey
	}{
		{
			name:    "standard GET",
			group:   &HandlerGroup{Server: "http", Prefix: "/api"},
			handler: &Handler{Method: "GET", Path: "/users"},
			expected: HandlerKey{Server: "http", Method: "GET", Path: "/api/users"},
		},
		{
			name:    "ANY method",
			group:   &HandlerGroup{Server: "http", Prefix: "/api"},
			handler: &Handler{Method: MethodAny, Path: "/proxy"},
			expected: HandlerKey{Server: "http", Method: "ANY", Path: "/api/proxy"},
		},
		{
			name:    "wildcard path",
			group:   &HandlerGroup{Server: "http", Prefix: "/api"},
			handler: &Handler{Method: "GET", Path: "/*"},
			expected: HandlerKey{Server: "http", Method: "GET", Path: "/api/*"},
		},
		{
			name:    "ANY with wildcard",
			group:   &HandlerGroup{Server: "http", Prefix: "/proxy"},
			handler: &Handler{Method: MethodAny, Path: "/*"},
			expected: HandlerKey{Server: "http", Method: "ANY", Path: "/proxy/*"},
		},
		{
			name:    "nil group",
			group:   nil,
			handler: &Handler{Method: "GET", Path: "/health"},
			expected: HandlerKey{Server: "", Method: "GET", Path: "/health"},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.expected, NewHandlerKey(tt.group, tt.handler))
		})
	}
}
