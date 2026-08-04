package supervisor

import (
	"sync"

	"github.com/xhanio/framingo/pkg/types/common"
	"github.com/xhanio/framingo/pkg/types/entity"
)

// statsStore zips the per-service records with the lock that guards them:
// the monitor goroutine writes probe results while health endpoints read
// them. All access goes through track, snapshot, and update.
type statsStore struct {
	sync.RWMutex
	records map[string]*entity.SupervisorStats
}

func newStatsStore() *statsStore {
	return &statsStore{
		records: make(map[string]*entity.SupervisorStats),
	}
}

// track adds a record for the service if none exists yet, reporting whether
// one was added.
func (s *statsStore) track(service common.Service) bool {
	s.Lock()
	defer s.Unlock()
	if _, ok := s.records[service.Name()]; ok {
		return false
	}
	s.records[service.Name()] = &entity.SupervisorStats{
		Name:   service.Name(),
		Source: service,
	}
	return true
}

// snapshot returns a copy of the named service's stats, or nil if the
// service is unknown. Copies are what leave the lock: callers read them
// freely while the monitor keeps writing the live record.
func (s *statsStore) snapshot(name string) *entity.SupervisorStats {
	s.RLock()
	defer s.RUnlock()
	stat, ok := s.records[name]
	if !ok {
		return nil
	}
	cp := *stat
	return &cp
}

// update mutates the named service's stats under the write lock. Blocking
// work (probes, Init/Start/Stop) stays outside fn.
func (s *statsStore) update(name string, fn func(stat *entity.SupervisorStats)) {
	s.Lock()
	defer s.Unlock()
	if stat, ok := s.records[name]; ok {
		fn(stat)
	}
}
