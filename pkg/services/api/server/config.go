package server

import (
	"fmt"
	"path"

	"github.com/xhanio/errors"
	"gopkg.in/yaml.v3"
)

// handlerKey uniquely identifies a handler within a server.
type handlerKey struct {
	Server string
	Method string
	Path   string
}

func (k handlerKey) String() string {
	return fmt.Sprintf("[%s] %s %s", k.Server, k.Method, k.Path)
}

// newHandlerKey creates a handlerKey from a handlerGroupConfig and handlerConfig.
func newHandlerKey(g *handlerGroupConfig, h *handlerConfig) handlerKey {
	var server, prefix string
	if g != nil {
		server = g.Server
		prefix = g.Prefix
	}
	return handlerKey{
		Server: server,
		Method: h.Method,
		Path:   path.Join(prefix, h.Path),
	}
}

// handlerGroupConfig is the schema of a router.yaml. It is private to the
// server: routers hand their YAML over as bytes, middlewares receive theirs
// the same way, and the parsed form never crosses the package boundary.
type handlerGroupConfig struct {
	Server      string              `json:"server"` // default: http
	Prefix      string              `json:"prefix"` // default: /
	Handlers    []*handlerConfig    `json:"handlers"`
	Middlewares []*middlewareConfig `json:"middlewares"`
}

type handlerConfig struct {
	Method      string              `json:"method"`
	Path        string              `json:"path"`
	Middlewares []*middlewareConfig `json:"middlewares"`
	Permission  string              `json:"permission"`
	Poll        bool                `json:"poll"`
	Func        string              `json:"func"`
}

// middlewareConfig names a middleware and carries the value written under it,
// already split into its two meanings: a boolean is the switch, anything else
// is the config. In router.yaml an entry is a bare name or a single-key
// mapping:
//
//	middlewares:
//	  - authnuser          # bare: no switch, no config
//	  - authz: false       # boolean: Enabled, the switch
//	  - throttle:          # block: Config holds the raw YAML
//	      rps: 1
//	      burst_size: 3
//
// Parsed once, here - the resolution layer never re-inspects the bytes.
type middlewareConfig struct {
	Name    string
	Enabled *bool  // a boolean value under the name; nil when none
	Config  []byte // a non-boolean value under the name; nil when none
}

func (mc *middlewareConfig) UnmarshalYAML(node *yaml.Node) error {
	switch node.Kind {
	case yaml.ScalarNode:
		return node.Decode(&mc.Name)
	case yaml.MappingNode:
		var entry map[string]yaml.Node
		if err := node.Decode(&entry); err != nil {
			return errors.Wrap(err)
		}
		if len(entry) != 1 {
			return errors.Newf("a middleware entry must map exactly one name to its config")
		}
		for name, value := range entry {
			mc.Name = name
			enabled, raw, err := splitMiddlewareValue(&value)
			if err != nil {
				return err
			}
			mc.Enabled, mc.Config = enabled, raw
		}
		return nil
	default:
		return errors.Newf("a middleware entry must be a name or a single-key mapping")
	}
}

// splitMiddlewareValue reads the value under a middleware's name as either
// the switch (a YAML boolean - a quoted "false" is a string and stays
// config) or the config bytes.
func splitMiddlewareValue(value *yaml.Node) (*bool, []byte, error) {
	if value.Kind == yaml.ScalarNode && value.Tag == "!!bool" {
		var b bool
		if err := value.Decode(&b); err != nil {
			return nil, nil, errors.Wrap(err)
		}
		return &b, nil, nil
	}
	raw, err := configBytes(value)
	if err != nil {
		return nil, nil, err
	}
	return nil, raw, nil
}

// configBytes renders the YAML written under a middleware's name back to the
// raw bytes Middleware.Func receives. A null value - a name with nothing
// under it - is no config at all, not the string "null". This is the one
// place the parsed tree turns back into bytes: the contract stays []byte so
// middlewares owe nothing to the YAML library, at the price of this
// re-marshal.
func configBytes(node *yaml.Node) ([]byte, error) {
	if node.Tag == "!!null" {
		return nil, nil
	}
	raw, err := yaml.Marshal(node)
	if err != nil {
		return nil, errors.Wrap(err)
	}
	return raw, nil
}
