package plan

import (
	"context"
	"testing"

	"github.com/xhanio/framingo/pkg/services/supervisor"
	"github.com/xhanio/framingo/pkg/services/task"
	"github.com/xhanio/framingo/pkg/types/entity"
	"github.com/xhanio/framingo/pkg/utils/log"
)

// The task manager restarted alone under the plan manager, as the supervisor
// restarts it: a firing it dropped ends canceled, and every plan's task is
// taken again.
func TestATaskManagerRestartUnderThePlans(t *testing.T) {
	tm := task.New(task.MaxConcurrency(1))
	sender := &recordingSender{}
	m := newManager(sender, tm, WithLogDir(t.TempDir()))
	sup := supervisor.New(nil, supervisor.WithLogger(log.New(log.WithLevel(-1))))
	if err := sup.Register(tm, m); err != nil {
		t.Fatal(err)
	}
	if err := sup.TopoSort(); err != nil {
		t.Fatal(err)
	}
	if err := sup.Init(context.Background()); err != nil {
		t.Fatal(err)
	}
	if err := sup.Start(context.Background()); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = sup.Stop(context.Background()) })
	if err := m.Add(commandPlan("first", &entity.Task{}, "sleep", "30")); err != nil {
		t.Fatal(err)
	}
	if err := m.Add(commandPlan("second", &entity.Task{}, "true")); err != nil {
		t.Fatal(err)
	}
	trigger(t, m, "first")
	eventually(t, "the first run to start", func() bool { return len(sender.started()) == 1 })
	queued := trigger(t, m, "second") // held back by the one slot

	if err := sup.RestartService(context.Background(), tm.Name()); err != nil {
		t.Fatal(err)
	}
	assertCanceledBeforeItStarted(t, sender, queued)

	again := trigger(t, m, "second")
	eventually(t, "the second plan to run on the restarted task manager", func() bool {
		_, finished := sender.run(again)
		return finished != nil
	})
	if _, finished := sender.run(again); finished.State != entity.PlanRunSucceeded {
		t.Fatalf("finished = %+v", finished)
	}
}
