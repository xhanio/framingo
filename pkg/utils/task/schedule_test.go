package task

import (
	"context"
	"testing"

	"github.com/xhanio/framingo/pkg/utils/cronutil"
	"github.com/xhanio/framingo/pkg/utils/job"
)

func TestValidScheduleIsAcceptedByTheManager(t *testing.T) {
	s := newManager()
	for _, spec := range []string{"0 30 6 * * *", "*/5 * * * *", "@daily", "@every 10m"} {
		if err := cronutil.Validate(spec); err != nil {
			t.Fatalf("cronutil.Validate(%q) = %v", spec, err)
		}
		tk := &Task{Job: job.New(spec, func(job.Context) error { return nil }), Ctx: context.Background(), Schedule: spec}
		if err := s.Add(tk); err != nil {
			t.Fatalf("Add rejected %q that validation accepted: %v", spec, err)
		}
	}
}
