package auth

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/xhanio/framingo/example/pkg/types/entity"
	"github.com/xhanio/framingo/example/pkg/types/message"
	"github.com/xhanio/framingo/example/pkg/types/model"
	"github.com/xhanio/framingo/example/pkg/types/preset"
	"github.com/xhanio/framingo/pkg/types/common"
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

// a lock-order inversion shows only as wedged goroutines, hence the timeout
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

func TestSessionOperationsDoNotInvertLockOrder(t *testing.T) {
	m := newManager(nil, nil, nil, nil)
	sess := newSession(t, m, "alice")

	mustFinish(t, 5*time.Second, "repeated CloseSession", func() {
		m.CloseSession(context.Background(), sess.ID)
		m.CloseSession(context.Background(), sess.ID)
	})

	mustFinish(t, 3*time.Second, "GetSession after close", func() {
		m.GetSession(context.Background(), sess.ID)
	})
}

func TestConcurrentSessionOperations(t *testing.T) {
	m := newManager(nil, nil, nil, nil)
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

// guards entity.Credential's immutability under -race: GetSession shares the stored pointer
func TestSharedSessionAccessIsRaceFree(t *testing.T) {
	m := newManager(nil, nil, nil, nil)
	sess := newSession(t, m, "carol")

	var wg sync.WaitGroup
	for i := 0; i < 8; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := 0; j < 200; j++ {
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
	m := newManager(nil, nil, nil, nil)
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

func TestAccountEventsRevokeLocalUserSessions(t *testing.T) {
	m := newManager(nil, nil, nil, nil)
	open := func(userID int32, source string) *entity.Session {
		m.Lock()
		defer m.Unlock()
		return m.createSession(context.Background(), &entity.Credential{
			Source:           source,
			OrganizationName: "not-the-default",
			UserName:         "renamed-since",
			UserID:           userID,
		})
	}
	alive := func(s *entity.Session) bool {
		_, ok := m.GetSession(context.Background(), s.ID)
		return ok
	}
	deleted := open(2, preset.AuthSourceLocalUser)
	reset := open(3, preset.AuthSourceLocalUser)
	changed := open(4, preset.AuthSourceLocalUser)
	bystander := open(5, preset.AuthSourceLocalUser)
	token := open(2, preset.AuthSourceAPIToken) // same number, not a local user
	time.Sleep(50 * time.Millisecond)           // let the lease goroutines reach their select

	ctx := context.Background()
	require.NoError(t, m.HandleMessage(ctx, message.LocalUsersDeleted{UserIDs: []int32{2}}))
	require.NoError(t, m.HandleMessage(ctx, message.LocalUserPasswordReset{UserID: 3}))
	require.NoError(t, m.HandleMessage(ctx, message.LocalUserIdentityChanged{UserID: 4}))

	for _, s := range []*entity.Session{deleted, reset, changed} {
		assert.Eventually(t, func() bool { return !alive(s) }, time.Second, 10*time.Millisecond,
			"session of user %d survived its account event", s.Credential.UserID)
	}
	assert.True(t, alive(bystander), "an unrelated user's session was ended")
	assert.True(t, alive(token), "a non-local credential with a matching number was ended")
}

type fakeLDAP struct{ model.LDAPAuthN }

func (fakeLDAP) Name() string { return "ldap" }

func TestOptionalSourcesAreDependencies(t *testing.T) {
	ldap := fakeLDAP{}
	deps := newManager(nil, nil, ldap, nil).Dependencies()
	assert.Contains(t, deps, common.Service(ldap), "a wired LDAP source")
	assert.Contains(t, deps, common.Service(nil), "the unwired API-token source, skipped by the supervisor")
}
