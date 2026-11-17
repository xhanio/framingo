package infra

import (
	"os/exec"
	"testing"
	"time"
)

// GetTimezone reads timedatectl, which only systemd hosts have; elsewhere
// there is nothing to test.
func TestGetTimezoneLoadsAsALocation(t *testing.T) {
	if _, err := exec.LookPath("timedatectl"); err != nil {
		t.Skip("timedatectl not available:", err)
	}
	tz, err := GetTimezone()
	if err != nil {
		t.Fatal(err)
	}
	loc, err := time.LoadLocation(tz)
	if err != nil {
		t.Fatalf("timezone %q does not load: %v", tz, err)
	}
	t.Logf("(GMT %s) %s", time.Now().In(loc).Format("-07:00"), loc)
}
