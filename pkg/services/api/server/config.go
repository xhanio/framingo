package server

import (
	"bytes"
	"fmt"
	"io"
	"path"

	"github.com/xhanio/errors"
	"gopkg.in/yaml.v3"
)

type handlerKey struct {
	Server string
	Method string
	Path   string
}

func (k handlerKey) String() string {
	return fmt.Sprintf("[%s] %s %s", k.Server, k.Method, k.Path)
}

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

// handlerGroupConfig is one router.yaml document; it never leaves this package.
type handlerGroupConfig struct {
	Server      string              `json:"server"` // required: a name given to Add
	Prefix      string              `json:"prefix"` // default: /
	Handlers    []*handlerConfig    `json:"handlers"`
	Middlewares []*middlewareConfig `json:"middlewares"`
}

// one handler group per `---` document; empty documents are skipped, none at all is an error
func parseRouterConfig(data []byte) ([]*handlerGroupConfig, error) {
	dec := yaml.NewDecoder(bytes.NewReader(data))
	var groups []*handlerGroupConfig
	for doc := 1; ; doc++ {
		var g *handlerGroupConfig
		err := dec.Decode(&g)
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			return nil, errors.Wrapf(err, "document %d", doc)
		}
		if g != nil {
			groups = append(groups, g)
		}
	}
	if len(groups) == 0 {
		return nil, errors.Newf("no handler group in router config")
	}
	return groups, nil
}

type handlerConfig struct {
	Method      string              `json:"method"`
	Path        string              `json:"path"`
	Middlewares []*middlewareConfig `json:"middlewares"`
	Permission  string              `json:"permission"`
	Poll        bool                `json:"poll"`
	Func        string              `json:"func"`
}

// middlewareConfig is a bare name or a single-key mapping: a boolean value is the switch, anything else the config.
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

// a quoted "false" is a string, so it stays config
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

// a null value is no config, not "null"; re-marshalled so Middleware.Func stays []byte and YAML-free
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
