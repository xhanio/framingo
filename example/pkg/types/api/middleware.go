package api

// Each middleware unmarshals its own config block; a boolean is its switch and arrives as Func's enabled, never as bytes.

// ThrottleConfig: zeros mean unthrottled.
type ThrottleConfig struct {
	RPS       float64 `yaml:"rps"`
	BurstSize int     `yaml:"burst_size"`
}

// CORSConfig: empty fields keep echo's permissive defaults; AllowCredentials requires
// explicit AllowOrigins, since browsers reject credentials against a wildcard.
type CORSConfig struct {
	AllowOrigins     []string `yaml:"allow_origins"`
	AllowMethods     []string `yaml:"allow_methods"`
	AllowHeaders     []string `yaml:"allow_headers"`
	AllowCredentials bool     `yaml:"allow_credentials"`
	MaxAge           int      `yaml:"max_age"`
}
