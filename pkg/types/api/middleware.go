package api

// CORSConfig is the policy block of the server's built-in cors middleware,
// written in YAML under "cors" in the server's middleware configs. Empty
// fields keep echo's permissive defaults. Unknown fields fail startup, and
// AllowCredentials requires explicit AllowOrigins - browsers reject
// credentials against a wildcard origin, so that combination never works and
// is refused.
type CORSConfig struct {
	AllowOrigins     []string `yaml:"allow_origins"`
	AllowMethods     []string `yaml:"allow_methods"`
	AllowHeaders     []string `yaml:"allow_headers"`
	AllowCredentials bool     `yaml:"allow_credentials"`
	MaxAge           int      `yaml:"max_age"`
}
