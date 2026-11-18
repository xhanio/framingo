package auth

import (
	"context"
	"io"
	"time"

	"github.com/xhanio/framingo/pkg/structs/lease"
	"github.com/xhanio/framingo/pkg/types/common"
	"github.com/xhanio/framingo/pkg/utils/printutil"

	"github.com/xhanio/framingo/example/pkg/types/message"
)

func (m *manager) Info(w io.Writer, debug bool) {
	if !debug {
		return
	}
	// snapshot under the lock; lease methods must not run while m is held (see lookupSession)
	type row struct {
		sid   string
		cred  string
		lease lease.Lease
	}
	m.RLock()
	users := make(map[string]int, len(m.users))
	for cid, sessions := range m.users {
		users[cid] = len(sessions)
	}
	rows := make([]row, 0, len(m.sessions))
	for sid, session := range m.sessions {
		rows = append(rows, row{sid: sid, cred: session.Credential.UID(), lease: session.Lease})
	}
	m.RUnlock()

	t := printutil.NewTable(w)
	t.Header(m.Name())
	t.Title("Credential", "Sessions")
	for cid, n := range users {
		t.Row(cid, n)
	}
	t.NewLine()
	t.Title("SessionID", "Credential", "Expired", "ExpiresAt")
	for _, r := range rows {
		t.Row(r.sid, r.cred, r.lease.Expired(), r.lease.ExpiresAt().In(time.Local).Format(common.TimeFormat))
	}
	t.NewLine()
	t.Flush()
}

// HandleMessage: a session must not outlive the deletion, password or identity it was issued against.
func (m *manager) HandleMessage(ctx context.Context, e common.Message) error {
	switch evt := e.(type) {
	case message.LocalUsersDeleted:
		m.revokeLocalUsers(evt.UserIDs...)
	case message.LocalUserPasswordReset:
		m.revokeLocalUsers(evt.UserID)
	case message.LocalUserIdentityChanged:
		m.revokeLocalUsers(evt.UserID)
	}
	return nil
}
