package cronutil

import (
	"testing"

	"github.com/xhanio/errors"
)

func TestValidate(t *testing.T) {
	tests := []struct {
		spec string
		ok   bool
	}{
		{"*/5 * * * *", true},
		{"30 6 * * 1-5", true},
		{"0 30 6 * * *", true},
		{"CRON_TZ=Asia/Shanghai 30 6 * * *", true},
		{"@daily", true},
		{"@hourly", true},
		{"@every 1h30m", true},
		{"CRON_TZ=Asia/Shanghai @daily", true},
		{"@sometimes", false},
		{"@every forever", false},
		{"not a cron", false},
		{"* * *", false},
		{"60 * * * *", false},
		{"* * * * * * *", false},
		{"", false},
	}
	for _, tt := range tests {
		err := Validate(tt.spec)
		if (err == nil) != tt.ok {
			t.Errorf("Validate(%q) = %v, want ok=%v", tt.spec, err, tt.ok)
		}
		if err != nil && !errors.Is(err, errors.InvalidArgument) {
			t.Errorf("Validate(%q) = %v, want an InvalidArgument", tt.spec, err)
		}
	}
}
