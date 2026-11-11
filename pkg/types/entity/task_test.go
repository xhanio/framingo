package entity

import (
	"testing"
	"time"
)

// A Task is configuration, so it copies like any value; go vet's copylocks check flags this
// if the manager keeps state in it again.
func TestTaskIsAPlainValue(t *testing.T) {
	a := Task{Schedule: "@daily", Priority: 3}
	b := a
	b.Priority = 4
	if a.Priority != 3 {
		t.Fatal("changing a copy changed the original")
	}
}

func TestGraceDefaultsToTenSeconds(t *testing.T) {
	cases := []struct {
		period time.Duration
		want   time.Duration
	}{
		{period: 0, want: 10 * time.Second},
		{period: -time.Second, want: 10 * time.Second},
		{period: time.Second, want: time.Second},
	}
	for _, tc := range cases {
		if got := (&Task{GracePeriod: tc.period}).Grace(); got != tc.want {
			t.Errorf("Grace of %s = %s, want %s", tc.period, got, tc.want)
		}
	}
}
