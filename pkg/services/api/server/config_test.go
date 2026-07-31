package server

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gopkg.in/yaml.v3"

	"github.com/xhanio/framingo/pkg/types/api"
)

func TestNewHandlerKey(t *testing.T) {
	tests := []struct {
		name     string
		group    *handlerGroupConfig
		handler  *handlerConfig
		expected handlerKey
	}{
		{
			name:     "standard GET",
			group:    &handlerGroupConfig{Server: "http", Prefix: "/api"},
			handler:  &handlerConfig{Method: "GET", Path: "/users"},
			expected: handlerKey{Server: "http", Method: "GET", Path: "/api/users"},
		},
		{
			name:     "ANY method",
			group:    &handlerGroupConfig{Server: "http", Prefix: "/api"},
			handler:  &handlerConfig{Method: "ANY", Path: "/proxy"},
			expected: handlerKey{Server: "http", Method: "ANY", Path: "/api/proxy"},
		},
		{
			name:     "wildcard path",
			group:    &handlerGroupConfig{Server: "http", Prefix: "/api"},
			handler:  &handlerConfig{Method: "GET", Path: "/*"},
			expected: handlerKey{Server: "http", Method: "GET", Path: "/api/*"},
		},
		{
			name:     "ANY with wildcard",
			group:    &handlerGroupConfig{Server: "http", Prefix: "/proxy"},
			handler:  &handlerConfig{Method: "ANY", Path: "/*"},
			expected: handlerKey{Server: "http", Method: "ANY", Path: "/proxy/*"},
		},
		{
			name:     "nil group",
			group:    nil,
			handler:  &handlerConfig{Method: "GET", Path: "/health"},
			expected: handlerKey{Server: "", Method: "GET", Path: "/health"},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.expected, newHandlerKey(tt.group, tt.handler))
		})
	}
}

func TestMiddlewareConfig_UnmarshalYAML(t *testing.T) {
	unmarshal := func(t *testing.T, s string) []*middlewareConfig {
		t.Helper()
		var out struct {
			Middlewares []*middlewareConfig `yaml:"middlewares"`
		}
		require.NoError(t, yaml.Unmarshal([]byte(s), &out))
		return out.Middlewares
	}

	t.Run("bare name", func(t *testing.T) {
		refs := unmarshal(t, "middlewares:\n  - authnuser\n")
		require.Len(t, refs, 1)
		assert.Equal(t, "authnuser", refs[0].Name)
		assert.Nil(t, refs[0].Config)
	})

	t.Run("name with config", func(t *testing.T) {
		refs := unmarshal(t, "middlewares:\n  - throttle:\n      rps: 1\n      burst_size: 3\n")
		require.Len(t, refs, 1)
		assert.Equal(t, "throttle", refs[0].Name)
		var cfg struct {
			RPS   float64 `yaml:"rps"`
			Burst int     `yaml:"burst_size"`
		}
		require.NoError(t, yaml.Unmarshal(refs[0].Config, &cfg))
		assert.Equal(t, 1.0, cfg.RPS)
		assert.Equal(t, 3, cfg.Burst)
	})

	t.Run("name with a trailing colon and nothing under it is bare", func(t *testing.T) {
		refs := unmarshal(t, "middlewares:\n  - authnuser:\n")
		require.Len(t, refs, 1)
		assert.Equal(t, "authnuser", refs[0].Name)
		assert.Nil(t, refs[0].Config)
	})

	t.Run("two names in one mapping is an error", func(t *testing.T) {
		var out struct {
			Middlewares []*middlewareConfig `yaml:"middlewares"`
		}
		err := yaml.Unmarshal([]byte("middlewares:\n  - a: 1\n    b: 2\n"), &out)
		assert.Error(t, err)
	})

	t.Run("a sequence entry is an error", func(t *testing.T) {
		var out struct {
			Middlewares []*middlewareConfig `yaml:"middlewares"`
		}
		err := yaml.Unmarshal([]byte("middlewares:\n  - [a, b]\n"), &out)
		assert.Error(t, err)
	})

	t.Run("mixed forms in one list", func(t *testing.T) {
		refs := unmarshal(t, "middlewares:\n  - authnuser\n  - throttle:\n      rps: 5\n  - authz\n")
		require.Len(t, refs, 3)
		assert.Equal(t, "authnuser", refs[0].Name)
		assert.Nil(t, refs[0].Config)
		assert.Equal(t, "throttle", refs[1].Name)
		assert.NotNil(t, refs[1].Config)
		assert.Equal(t, "authz", refs[2].Name)
		assert.Nil(t, refs[2].Config)
	})
}

func TestRequestKey(t *testing.T) {
	tests := []struct {
		name     string
		req      *api.RequestInfo
		prefix   string
		expected handlerKey
	}{
		{
			name:     "simple path",
			req:      &api.RequestInfo{Server: "http", Method: "GET", RawPath: "/api/users"},
			prefix:   "/",
			expected: handlerKey{Server: "http", Method: "GET", Path: "/api/users"},
		},
		{
			name:     "with endpoint prefix",
			req:      &api.RequestInfo{Server: "http", Method: "POST", RawPath: "/v1/api/users"},
			prefix:   "/v1",
			expected: handlerKey{Server: "http", Method: "POST", Path: "/api/users"},
		},
		{
			name:     "wildcard pattern",
			req:      &api.RequestInfo{Server: "http", Method: "GET", RawPath: "/api/*"},
			prefix:   "/",
			expected: handlerKey{Server: "http", Method: "GET", Path: "/api/*"},
		},
		{
			name:     "wildcard with endpoint prefix",
			req:      &api.RequestInfo{Server: "http", Method: "POST", RawPath: "/v1/proxy/*"},
			prefix:   "/v1",
			expected: handlerKey{Server: "http", Method: "POST", Path: "/proxy/*"},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s := &server{endpoint: &api.Endpoint{Path: tt.prefix}}
			assert.Equal(t, tt.expected, s.requestKey(tt.req))
		})
	}
}
