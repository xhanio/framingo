package osutil

import (
	"os/exec"
	"testing"
	"time"
)

// timedatectl exists only on systemd hosts
func TestTimezoneLoadsAsALocation(t *testing.T) {
	if _, err := exec.LookPath("timedatectl"); err != nil {
		t.Skip("timedatectl not available:", err)
	}
	loc, err := Timezone()
	if err != nil {
		t.Fatal(err)
	}
	t.Logf("(GMT %s) %s", time.Now().In(loc).Format("-07:00"), loc)
}
