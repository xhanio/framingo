package task

import (
	"context"
	"sync"

	"github.com/robfig/cron/v3"

	"github.com/xhanio/framingo/pkg/structs/staque"
	"github.com/xhanio/framingo/pkg/types/common"
	"github.com/xhanio/framingo/pkg/utils/job"
	"github.com/xhanio/framingo/pkg/utils/log"
	"github.com/xhanio/framingo/pkg/utils/syncutil"
)

var _ Manager = (*manager)(nil)

type manager struct {
	log log.Logger

	name string

	cm *cron.Cron
	pq staque.Priority[*registration]

	concurrent int
	el         *sync.RWMutex // lock for registered and running
	registered map[string]*registration
	running    map[job.Job]*execution // by job: a job runs once at a time, whichever copy of its task started it
	arrivals   uint64                 // numbers each entry into the queue, for first come first served
	executions syncutil.WaitGroup     // whichever Start they came from

	cancel context.CancelFunc
	looped chan struct{} // closed as the loop of the latest Start returns
}

func New(opts ...Option) Manager {
	return newManager(opts...)
}

func newManager(opts ...Option) *manager {
	m := &manager{
		el:         &sync.RWMutex{},
		registered: make(map[string]*registration),
		running:    make(map[job.Job]*execution),
	}
	m.applyOptions(opts...)
	m.setDefaults()
	m.pq = staque.NewPriority(
		staque.WithLessFunc(queueOrder),
		staque.WithLogger[*registration](m.log),
	)
	return m
}

func (m *manager) Name() string {
	return m.name
}

func (m *manager) Dependencies() []common.Service {
	return nil
}
