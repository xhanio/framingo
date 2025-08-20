package executor

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/xhanio/framingo/pkg/utils/errors"
	"github.com/xhanio/framingo/pkg/utils/job"
)

func TestJobRetry(t *testing.T) {
	j := job.New("", job.Wrap(func(ctx context.Context) error {
		for {
			select {
			case <-ctx.Done():
				if ctx.Err() == context.DeadlineExceeded {
					return errors.Newf("job timed out")
				} else {
					return errors.Newf("job canceled")
				}
			case <-time.After(2 * time.Second):
				return errors.Newf("error occurred")
			}
		}
	}))

	t.Logf("job id is %s", j.ID())
	je := New(j, WithRetry(2, 1*time.Second))
	err := je.Start(context.Background(), nil)
	if err == nil {
		t.Fatal("failed job completed without error")
	}
	if !strings.Contains(err.Error(), "error occurred") {
		t.Fatalf("failed job completed with incorrect error: %v", j.Err())
	}
}

func TestJobTimedOut(t *testing.T) {
	j := job.New("", job.Wrap(func(ctx context.Context) error {
		for {
			select {
			case <-ctx.Done():
				if ctx.Err() == context.DeadlineExceeded {
					return errors.Newf("job timed out")
				} else {
					return errors.Newf("job canceled")
				}
			case <-time.After(2 * time.Second):
				return nil
			}
		}
	}))
	t.Logf("job id is %s", j.ID())
	je := New(j, WithTimeout(1*time.Second))
	err := je.Start(context.Background(), nil)
	if err == nil || err.Error() != "job timed out" {
		t.Fatal("job completed without error or with incorrect err message")
	}
}

func TestJobCanceled(t *testing.T) {
	j := job.New("", job.Wrap(func(ctx context.Context) error {
		for {
			select {
			case <-ctx.Done():
				if ctx.Err() == context.DeadlineExceeded {
					return errors.Newf("job timed out")
				} else {
					return errors.Newf("job canceled")
				}
			case <-time.After(4 * time.Second):
				return nil
			}
		}
	}))
	t.Logf("job id is %s", j.ID())
	je := New(j, WithTimeout(3*time.Second))
	go func() {
		<-time.After(2 * time.Second)
		j.Cancel()
	}()
	err := je.Start(context.Background(), nil)
	if err == nil || !j.IsState(job.StateCanceled) {
		t.Fatalf("job completed without error or with incorrect job state %s and error %s", j.State(), j.Err().Error())
	}
}
