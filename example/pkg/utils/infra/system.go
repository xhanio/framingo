package infra

import (
	"time"
)

// Process-wide facts, set once at startup by the server component.
var (
	Debug        bool
	StartTime    time.Time
	SerialNumber string
	ConfigDir    string
	EnvPrefix    string
)
