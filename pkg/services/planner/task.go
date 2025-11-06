package planner

import (
	"fmt"
	"time"

	"github.com/xhanio/errors"
	"github.com/xhanio/framingo/pkg/utils/cmdutil"
	"github.com/xhanio/framingo/pkg/utils/job"
	"k8s.io/apimachinery/pkg/labels"
)

func NewBashJob(caller string, metadata labels.Set, bin string, params []string) job.Job {
	return job.New(fmt.Sprintf("system.bash.%s", time.Now().Format(time.RFC3339)), func(ctx job.Context) error {
		cmd := cmdutil.New(bin, params,
			cmdutil.WithContext(ctx.Context()),
		)
		err := cmd.Start()
		if err != nil {
			return errors.Wrap(err)
		}
		ctx.SetResult(cmd.Output())
		return nil
	},
		job.WithLabels(metadata),
	)
}

// TODO
func NewAsyncBashJob(caller string, metadata labels.Set, bin string, params []string) job.Job {
	return job.New(fmt.Sprintf("system.bash_async.%s", time.Now().Format(time.RFC3339)), func(ctx job.Context) error {
		cmd := cmdutil.New(bin, params,
			cmdutil.WithContext(ctx.Context()),
			cmdutil.Async(),
			cmdutil.WithInput(),
		)
		err := cmd.Start()
		if err != nil {
			return errors.Wrap(err)
		}
		err = cmd.Wait()
		if err != nil {
			return errors.Wrap(err)
		}
		cmd.Print()
		ctx.SetResult(cmd.Output())
		return nil
	},
		job.WithLabels(metadata),
	)
}
