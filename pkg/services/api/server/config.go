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

// middlewareConfig names a middleware and carries the raw YAML written under
// it. In router.yaml an entry is either a bare name or a single-key mapping:
//
//	middlewares:
//	  - authnuser          # bare: Config is nil
//	  - throttle:          # configured: Config holds the block's YAML
//	      rps: 1
//	      burst_size: 3
type middlewareConfig struct {
	Name   string
	Config []byte
}

func (mc *middlewareConfig) UnmarshalYAML(node *yaml.Node) error {
	switch node.Kind {
	case yaml.ScalarNode:
		return node.Decode(&mc.Name)
	case yaml.MappingNode:
		if len(node.Content) != 2 {
			return errors.Newf("a middleware entry must map exactly one name to its config")
		}
		if err := node.Content[0].Decode(&mc.Name); err != nil {
			return errors.Wrap(err)
		}
		value := node.Content[1]
		// "- name:" with nothing under it parses as a mapping to null; treat
		// it as a bare attachment rather than handing the middleware "null".
		if value.Tag == "!!null" {
			return nil
		}
		raw, err := yaml.Marshal(value)
		if err != nil {
			return errors.Wrap(err)
		}
		mc.Config = raw
		return nil
	default:
		return errors.Newf("a middleware entry must be a name or a single-key mapping")
	}
}
