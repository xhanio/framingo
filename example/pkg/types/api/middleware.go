package api

// Middleware config blocks: the YAML written under a middleware's name,
// whether at a route in router.yaml or in the server's middleware configs
// (api.<server>.middlewares in config.yaml). Each middleware unmarshals its
// own block; the framework carries it as raw bytes and never interprets it.

// ThrottleConfig is the throttle middleware's block. Zeros mean unthrottled.
type ThrottleConfig struct {
	RPS       float64 `yaml:"rps"`
	BurstSize int     `yaml:"burst_size"`
}
