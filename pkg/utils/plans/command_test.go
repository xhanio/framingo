package plans

import (
	"context"
	"errors"
	"io"
	"strings"
	"testing"
	"time"

	"github.com/xhanio/framingo/pkg/types/entity"
	"github.com/xhanio/framingo/pkg/utils/job"
	"github.com/xhanio/framingo/pkg/utils/jobs"
)

func commandPlan(cmd any, timeout time.Duration) *entity.Plan {
	return &entity.Plan{ID: "x", Kind: entity.PlanKindCommand, Params: cmd, Task: &entity.Task{Timeout: timeout}}
}

func TestNewCommandRejectsWhatIsNotACommand(t *testing.T) {
	cases := []struct {
		name   string
		params any
	}{
		{name: "params of another type", params: "true"},
		{name: "no argv", params: jobs.Command{}},
		{name: "an output of its own", params: jobs.Command{Argv: []string{"true"}, Out: io.Discard}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if _, err := NewCommand(commandPlan(tc.params, 0)); err == nil {
				t.Fatal("NewCommand = nil error")
			}
		})
	}
}

func TestCommandHeaderIsTheCommandLine(t *testing.T) {
	c, err := NewCommand(commandPlan(jobs.Command{Argv: []string{"sh", "-c", "true"}}, 0))
	if err != nil {
		t.Fatal(err)
	}
	if h := c.Header(); h != "sh -c true" {
		t.Fatalf("header = %q", h)
	}
}

// run runs c once as a job's function, with ctx as the job's context.
func run(ctx context.Context, c Command) (Outcome, string, error) {
	var out strings.Builder
	var o Outcome
	j := job.New("run", func(jc job.Context) error {
		var err error
		o, err = c.Run(jc, &out)
		return err
	})
	j.Run(ctx, nil)
	j.Wait()
	return o, out.String(), j.Err()
}

func TestCommandRunSaysHowTheRunEnded(t *testing.T) {
	cases := []struct {
		name    string
		script  string
		timeout time.Duration
		cancel  bool
		want    Outcome
	}{
		{name: "succeeded", script: "echo hi", want: Outcome{State: entity.PlanRunSucceeded}},
		{name: "exit status", script: "exit 3", want: Outcome{State: entity.PlanRunFailed, Error: "exit status 3", ExitCode: 3}},
		{name: "timed out", script: "sleep 30", timeout: 100 * time.Millisecond, want: Outcome{State: entity.PlanRunFailed, Error: "timed out after 100ms", ExitCode: -1}},
		{name: "canceled", script: "sleep 30", cancel: true, want: Outcome{State: entity.PlanRunCanceled, Error: "canceled", ExitCode: -1}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			c, err := NewCommand(commandPlan(jobs.Command{Argv: []string{"sh", "-c", tc.script}}, tc.timeout))
			if err != nil {
				t.Fatal(err)
			}
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			if tc.timeout > 0 {
				ctx, cancel = context.WithTimeout(ctx, tc.timeout)
				defer cancel()
			}
			if tc.cancel {
				time.AfterFunc(100*time.Millisecond, cancel)
			}
			o, _, err := run(ctx, c)
			if o != tc.want {
				t.Fatalf("outcome = %+v, want %+v", o, tc.want)
			}
			if (err == nil) != (tc.want.State == entity.PlanRunSucceeded) {
				t.Fatalf("job error = %v for %s", err, tc.want.State)
			}
		})
	}
}

func TestCommandRunWritesItsOutputToOut(t *testing.T) {
	c, err := NewCommand(commandPlan(jobs.Command{Argv: []string{"sh", "-c", "echo out; echo err >&2"}}, 0))
	if err != nil {
		t.Fatal(err)
	}
	if _, out, err := run(context.Background(), c); err != nil || out != "out\nerr\n" {
		t.Fatalf("output = %q, error = %v", out, err)
	}
}

func TestCommandRunErrorIsTheCommandsError(t *testing.T) {
	c, err := NewCommand(commandPlan(jobs.Command{Argv: []string{"sh", "-c", "exit 4"}}, 0))
	if err != nil {
		t.Fatal(err)
	}
	_, _, runErr := run(context.Background(), c)
	var failed *jobs.CommandError
	if !errors.As(runErr, &failed) || failed.ExitCode != 4 {
		t.Fatalf("job error = %v", runErr)
	}
}
