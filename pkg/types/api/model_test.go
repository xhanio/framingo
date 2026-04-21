package api

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestHandlerKey(t *testing.T) {
	tests := []struct {
		name     string
		group    *HandlerGroup
		handler  *Handler
		expected string
	}{
		{
			name:     "standard GET",
			group:    &HandlerGroup{Server: "http", Prefix: "/api"},
			handler:  &Handler{Method: "GET", Path: "/users"},
			expected: "http<GET>/api/users",
		},
		{
			name:     "ANY method",
			group:    &HandlerGroup{Server: "http", Prefix: "/api"},
			handler:  &Handler{Method: MethodAny, Path: "/proxy"},
			expected: "http<ANY>/api/proxy",
		},
		{
			name:     "wildcard path",
			group:    &HandlerGroup{Server: "http", Prefix: "/api"},
			handler:  &Handler{Method: "GET", Path: "/*"},
			expected: "http<GET>/api/*",
		},
		{
			name:     "ANY with wildcard",
			group:    &HandlerGroup{Server: "http", Prefix: "/proxy"},
			handler:  &Handler{Method: MethodAny, Path: "/*"},
			expected: "http<ANY>/proxy/*",
		},
		{
			name:     "nil group",
			group:    nil,
			handler:  &Handler{Method: "GET", Path: "/health"},
			expected: "<GET>/health",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.expected, HandlerKey(tt.group, tt.handler))
		})
	}
}
