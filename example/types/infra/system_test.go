package infra

import (
	"fmt"
	"testing"
	"time"
)

func TestFormatTimezone(t *testing.T) {
	tz, err := GetTimezone()
	if err != nil {
		t.Fatal(err)
	}
	loc, err := time.LoadLocation(tz)
	if err != nil {
		t.Fatal(err)
	}
	gmt := fmt.Sprintf("(GMT %s) %s", time.Now().In(loc).Format("-07:00"), loc)
	t.Log(gmt)
}
