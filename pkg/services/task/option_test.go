package task

import (
	"context"
	"sync/atomic"
	"testing"
	"time"

	"github.com/xhanio/framingo/pkg/types/entity"
)

func TestNewWithoutAConcurrencyStillRunsTasks(t *testing.T) {
	cases := []struct {
		name string
		opts []Option
	}{
		{name: "not set"},
		{name: "zero", opts: []Option{MaxConcurrency(0)}},
		{name: "negative", opts: []Option{MaxConcurrency(-1)}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			m := newManager(tc.opts...)
			if err := m.Start(context.Background()); err != nil {
				t.Fatal(err)
			}
			t.Cleanup(func() { _ = m.Stop(context.Background()) })
			if m.concurrent != defaultConcurrency {
				t.Fatalf("concurrency = %d, want %d", m.concurrent, defaultConcurrency)
			}
			var runs atomic.Int32
			if err := m.Add(&entity.Task{Job: countingJob("once", &runs)}); err != nil {
				t.Fatal(err)
			}
			eventually(t, "the task to run", func() bool { return runs.Load() == 1 })
		})
	}
}

// Schedules are read in local time unless the manager is given a location.
func TestWithLocationSetsTheScheduleZone(t *testing.T) {
	if got := newManager().cm.Location(); got != time.Local {
		t.Fatalf("default location = %v, want time.Local", got)
	}
	tokyo, err := time.LoadLocation("Asia/Tokyo")
	if err != nil {
		t.Skip("no zoneinfo:", err)
	}
	if got := newManager(WithLocation(tokyo)).cm.Location(); got != tokyo {
		t.Fatalf("location = %v, want %v", got, tokyo)
	}
}
