package api

// Middleware config blocks: the YAML written under a middleware's name,
// whether at a route in router.yaml or in the server's middleware configs
// (api.<server>.middlewares in config.yaml). Each middleware unmarshals its
// own block; the framework carries it as raw bytes and never interprets it -
// except a boolean, which is the middleware's switch and arrives as Func's
// enabled argument, never as bytes.

// ThrottleConfig is the throttle middleware's block. Zeros mean unthrottled.
type ThrottleConfig struct {
	RPS       float64 `yaml:"rps"`
	BurstSize int     `yaml:"burst_size"`
}

// CORSConfig is the cors middleware's policy block. Empty fields keep echo's
// permissive defaults. Unknown fields fail startup, and AllowCredentials
// requires explicit AllowOrigins - browsers reject credentials against a
// wildcard origin, so that combination never works and is refused.
type CORSConfig struct {
	AllowOrigins     []string `yaml:"allow_origins"`
	AllowMethods     []string `yaml:"allow_methods"`
	AllowHeaders     []string `yaml:"allow_headers"`
	AllowCredentials bool     `yaml:"allow_credentials"`
	MaxAge           int      `yaml:"max_age"`
}
