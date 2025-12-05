package lease

import (
	"sync"
	"time"

	"github.com/google/uuid"

	"github.com/xhanio/framingo/pkg/utils/log"
)

const (
	ActionTypeExtend = iota
	ActionTypeRefresh
	ActionTypeRenew
)

type action struct {
	Type      uint
	Duration  time.Duration
	ExpiresAt time.Time
}

type lease struct {
	id string

	log log.Logger

	duration time.Duration
	once     bool
	wall     bool

	sync.RWMutex
	expired   bool
	expiresAt time.Time
	ticker    *time.Ticker
	actionCh  chan action
	cancelCh  chan struct{}
	done      chan struct{} // signal that the loop has exited

	onCancel  []func()
	onExpire  []func()
	onRefresh []func()
	onExtend  []func()
	onRenew   []func()
}

func New(id string, duration time.Duration, opts ...LeaseOption) Lease {
	_id := id
	if _id == "" {
		_id = uuid.NewString()
	}
	l := &lease{
		log:      log.Default,
		id:       _id,
		duration: duration,

		onCancel:  make([]func(), 0),
		onExpire:  make([]func(), 0),
		onRefresh: make([]func(), 0),
		onExtend:  make([]func(), 0),
		onRenew:   make([]func(), 0),
	}
	for _, opt := range opts {
		opt(l)
	}
	return l
}

func (l *lease) initialize() {
	l.expired = false
	l.expiresAt = time.Now().Add(l.duration)
	l.actionCh = make(chan action, 1)
	l.cancelCh = make(chan struct{}, 1)
	l.done = make(chan struct{})
	l.ticker = time.NewTicker(100 * time.Millisecond)
}

func (l *lease) finalize() {
	l.ticker.Stop()
	l.ticker = nil
	l.expired = true
	// l.expiresAt = time.Time{} // keep last expiration history
}

func (l *lease) ID() string {
	return l.id
}

func (l *lease) Start() {
	l.RLock()
	if l.ticker != nil || (l.once && l.expired) {
		l.RUnlock()
		return
	}
	l.RUnlock()

	// fmt.Printf("starting %s\n", l.id)

	l.Lock()
	l.initialize()
	l.Unlock()

	// ensure done is closed when loop exits
	defer func() {
		l.Lock()
		close(l.done)
		l.Unlock()
	}()

	for {
		select {
		case <-l.cancelCh:
			l.Lock()
			l.finalize()
			for i := range l.onCancel {
				l.onCancel[i]()
			}
			l.Unlock()
			return
		case a := <-l.actionCh:
			l.Lock()
			switch a.Type {
			case ActionTypeRefresh:
				l.expiresAt = time.Now().Add(a.Duration)
				// l.log.Debugf("%s refreshed to %s", l.id, l.expiresAt.Local().Format("15:04:05.00"))
				for i := range l.onRefresh {
					l.onRefresh[i]()
				}
			case ActionTypeExtend:
				l.expiresAt = time.Now().Add(time.Until(l.expiresAt) + a.Duration)
				// l.log.Debugf("%s extended to %s", l.id, l.expiresAt.Local().Format("15:04:05.00"))
				for i := range l.onExtend {
					l.onExtend[i]()
				}
			case ActionTypeRenew:
				l.expiresAt = a.ExpiresAt
				// l.log.Debugf("%s renewed to %s", l.id, l.expiresAt.Local().Format("15:04:05.00"))
				for i := range l.onRenew {
					l.onRenew[i]()
				}
			}
			l.Unlock()
		case <-l.ticker.C:
			l.Lock()
			now := time.Now()
			if l.wall {
				now = now.Round(0)
			}
			if now.Sub(l.expiresAt) > 0 {
				l.finalize()
				for i := range l.onExpire {
					l.onExpire[i]()
				}
				// l.log.Debugf("%s expired at %s", l.id, l.expiresAt.Local().Format("15:04:05.00"))
				l.Unlock()
				// Do not close channels here to avoid race with writers
				return
			}
			l.Unlock()
		}
	}
}

func (l *lease) Refresh(duraton time.Duration) bool {
	l.RLock()
	if l.expired {
		l.RUnlock()
		return false
	}
	l.RUnlock()

	select {
	case l.actionCh <- action{
		Type:     ActionTypeRefresh,
		Duration: duraton,
	}:
		return true
	case <-l.done:
		// l.log.Debugf("%s refresh failed: lease closed", l.id)
		return false
	}
}

func (l *lease) Extend(duraton time.Duration) bool {
	l.RLock()
	if l.expired {
		l.RUnlock()
		return false
	}
	l.RUnlock()

	select {
	case l.actionCh <- action{
		Type:     ActionTypeExtend,
		Duration: duraton,
	}:
		return true
	case <-l.done:
		// l.log.Debugf("%s extend failed: lease closed", l.id)
		return false
	}
}

func (l *lease) Renew(expiresAt time.Time) bool {
	l.RLock()
	if l.expired {
		l.RUnlock()
		return false
	}
	l.RUnlock()

	select {
	case l.actionCh <- action{
		Type:      ActionTypeRenew,
		ExpiresAt: expiresAt,
	}:
		return true
	case <-l.done:
		// l.log.Debugf("%s renew failed: lease closed", l.id)
		return false
	}
}

func (l *lease) Cancel() {
	l.RLock()
	if l.expired {
		l.RUnlock()
		return
	}
	l.RUnlock()

	select {
	case l.cancelCh <- struct{}{}:
	case <-l.done:
		// l.log.Debugf("%s cancel failed: lease closed", l.id)
	}
}

func (l *lease) Expired() bool {
	l.RLock()
	defer l.RUnlock()
	return l.expired
}

func (l *lease) ExpiresAt() time.Time {
	l.RLock()
	defer l.RUnlock()
	return l.expiresAt
}

func (l *lease) OnExpired(fn func()) {
	l.Lock()
	defer l.Unlock()
	l.onExpire = append(l.onExpire, fn)
}

func (l *lease) OnCancel(fn func()) {
	l.Lock()
	defer l.Unlock()
	l.onCancel = append(l.onCancel, fn)
}

func (l *lease) OnRefresh(fn func()) {
	l.Lock()
	defer l.Unlock()
	l.onRefresh = append(l.onRefresh, fn)
}

func (l *lease) OnExtend(fn func()) {
	l.Lock()
	defer l.Unlock()
	l.onExtend = append(l.onExtend, fn)
}

func (l *lease) OnRenew(fn func()) {
	l.Lock()
	defer l.Unlock()
	l.onRenew = append(l.onRenew, fn)
}
