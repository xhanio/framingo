package auth

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/xhanio/framingo/example/pkg/types/entity"
)

func newSession(t *testing.T, m *manager, username string) *entity.Session {
	t.Helper()
	m.Lock()
	sess := m.createSession(context.Background(), &entity.Credential{
		Source:           "local-user",
		OrganizationName: "default",
		UserName:         username,
		Role:             "admin",
	})
	m.Unlock()
	time.Sleep(50 * time.Millisecond) // let the lease goroutine reach its select
	return sess
}

// mustFinish fails if fn has not returned within d, which is how a lock-order
// inversion presents: no panic, no error, just two wedged goroutines.
func mustFinish(t *testing.T, d time.Duration, what string, fn func()) {
	t.Helper()
	done := make(chan struct{})
	go func() { defer close(done); fn() }()
	select {
	case <-done:
	case <-time.After(d):
		t.Fatalf("%s did not return within %s - probable deadlock between the manager lock and the session lease lock", what, d)
	}
}

// The manager's lock and the lease's lock must never be acquired in opposite
// orders. The lease invokes OnCancel/OnExpired from its own goroutine while
// holding the lease lock, and those callbacks take the manager lock; so any
// manager method that calls into a lease must release its own lock first.
func TestSessionOperationsDoNotInvertLockOrder(t *testing.T) {
	m := newManager(nil, nil, nil)
	sess := newSession(t, m, "alice")

	mustFinish(t, 5*time.Second, "repeated CloseSession", func() {
		m.CloseSession(context.Background(), sess.ID)
		m.CloseSession(context.Background(), sess.ID)
	})

	// The manager must still be usable afterwards; if it wedged, every
	// authenticated request would hang here.
	mustFinish(t, 3*time.Second, "GetSession after close", func() {
		m.GetSession(context.Background(), sess.ID)
	})
}

func TestConcurrentSessionOperations(t *testing.T) {
	m := newManager(nil, nil, nil)
	sessions := make([]*entity.Session, 0, 4)
	for _, u := range []string{"a", "b", "c", "d"} {
		sessions = append(sessions, newSession(t, m, u))
	}

	mustFinish(t, 10*time.Second, "concurrent close/refresh/logout/info", func() {
		var wg sync.WaitGroup
		for _, s := range sessions {
			for i := 0; i < 4; i++ {
				wg.Add(1)
				go func(s *entity.Session) {
					defer wg.Done()
					m.RefreshSession(context.Background(), s.ID)
					m.CloseSession(context.Background(), s.ID)
					m.Logout(context.Background(), s.Credential)
					m.Info(discard{}, true)
				}(s)
			}
		}
		wg.Wait()
	})
}

type discard struct{}

func (discard) Write(p []byte) (int, error) { return len(p), nil }

// The session and its credential are shared by every request on that session,
// and GetSession hands out the stored pointer rather than a copy. That is only
// safe while entity.Credential stays immutable after construction — if request
// -scoped state (permissions, say) is ever written back onto it, this test
// starts failing under -race, which is the point.
func TestSharedSessionAccessIsRaceFree(t *testing.T) {
	m := newManager(nil, nil, nil)
	sess := newSession(t, m, "carol")

	var wg sync.WaitGroup
	for i := 0; i < 8; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := 0; j < 200; j++ {
				// what authnuser does per request
				if s, ok := m.GetSession(context.Background(), sess.ID); ok {
					_ = s.Credential.UID()
					_ = s.UID()
				}
				_ = m.List(entity.AuthListOptions{})
				m.Info(discard{}, true)
			}
		}()
	}
	wg.Wait()
}

func TestLogoutClosesEverySessionForTheUser(t *testing.T) {
	m := newManager(nil, nil, nil)
	s1 := newSession(t, m, "dave")
	s2 := newSession(t, m, "dave") // same user, second device

	if !m.Logout(context.Background(), s1.Credential) {
		t.Fatal("Logout reported nothing to close")
	}
	time.Sleep(300 * time.Millisecond) // lease goroutines unindex asynchronously

	for _, s := range []*entity.Session{s1, s2} {
		if _, ok := m.GetSession(context.Background(), s.ID); ok {
			t.Fatalf("session %s survived logout", s.ID)
		}
	}
}
