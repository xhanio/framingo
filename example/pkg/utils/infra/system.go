package infra

import (
	"time"
)

// Process-wide facts about this application, set once at startup by the
// server component. Facts about the host are not kept here: framingo's
// pkg/utils/osutil answers them when asked (osutil.Timezone), and the task
// manager takes the zone it schedules in as an option.
var (
	Debug        bool
	StartTime    time.Time
	SerialNumber string
	ConfigDir    string
	EnvPrefix    string
)
