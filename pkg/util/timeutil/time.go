package timeutil

import (
	"time"
)

var (
	// !! note that time.Unix(1<<63-1, 999999999) will result an overflow and break the Before/After logic
	// ref: https://github.com/golang/go/issues/39477
	MaxTime = time.Unix(1<<62, 999999999)
)

func Latest(ts ...time.Time) time.Time {
	var latest time.Time
	for _, t := range ts {
		if t.After(latest) {
			latest = t
		}
	}
	return latest
}

func Earliest(skipZeros bool, ts ...time.Time) time.Time {
	earliest := MaxTime
	for _, t := range ts {
		if t.IsZero() {
			if !skipZeros {
				return t
			}
			continue
		}
		if t.Before(earliest) {
			earliest = t
		}
	}
	return earliest
}
