package job

import (
	"context"
	"errors"
	"testing"
	"time"
)

func TestJobCompleted(t *testing.T) {
	j := New("", Wrap(func(ctx context.Context) error {
		for {
			select {
			case <-ctx.Done():
				if ctx.Err() == context.DeadlineExceeded {
					return errors.New("job timed out")
				} else {
					return errors.New("job canceled")
				}
			case <-time.After(2 * time.Second):
				return nil
			}
		}
	}))
	t.Logf("job id is %s", j.ID())
	j.Run(context.Background(), nil)
	j.Wait()
	if j.Err() != nil {
		t.Fatalf("completed job failed with error: %s", j.Err().Error())
	}
}

func TestJobPanicked(t *testing.T) {
	j := New("", Wrap(func(ctx context.Context) error {
		for {
			select {
			case <-ctx.Done():
				if ctx.Err() == context.DeadlineExceeded {
					return errors.New("job timed out")
				} else {
					return errors.New("job canceled")
				}
			case <-time.After(2 * time.Second):
				panic(errors.New("?!"))
			}
		}
	}))
	t.Logf("job id is %s", j.ID())
	j.Run(context.Background(), nil)
	j.Wait()
	if j.Err() == nil || j.Err().Error() != "?!" {
		t.Fatal("panicked job completed without error or with incorrect err message")
	}
}

func TestJobCanceled(t *testing.T) {
	j := New("", Wrap(func(ctx context.Context) error {
		for {
			select {
			case <-ctx.Done():
				if ctx.Err() == context.DeadlineExceeded {
					return errors.New("job timed out")
				} else {
					return errors.New("job canceled")
				}
			case <-time.After(4 * time.Second):
				return nil
			}
		}
	}))
	t.Logf("job id is %s", j.ID())
	go func() {
		<-time.After(2 * time.Second)
		j.Cancel()
	}()
	j.Run(context.Background(), nil)
	j.Wait()
	err := j.Err()
	if err == nil || !j.IsState(StateCanceled) {
		t.Fatalf("job completed without error or with incorrect job state %s and error %s", j.State(), j.Err().Error())
	}
}
