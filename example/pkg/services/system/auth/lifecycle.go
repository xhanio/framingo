package auth

import (
	"context"
	"io"

	"github.com/xhanio/framingo/pkg/structs/lease"
	"github.com/xhanio/framingo/pkg/types/common"
	"github.com/xhanio/framingo/pkg/utils/infra"
	"github.com/xhanio/framingo/pkg/utils/printutil"

	"github.com/xhanio/framingo/example/pkg/types/entity"
	"github.com/xhanio/framingo/example/pkg/types/message"
	"github.com/xhanio/framingo/example/pkg/types/preset"
)

func (m *manager) Info(w io.Writer, debug bool) {
	if !debug {
		return
	}
	// Snapshot under the lock; Lease.Expired/ExpiresAt take the lease's own
	// lock and must not be called while m is held (see lookupSession).
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
		t.Row(r.sid, r.cred, r.lease.Expired(), r.lease.ExpiresAt().In(infra.Timezone).Format(common.TimeFormat))
	}
	t.NewLine()
	t.Flush()
}

func (m *manager) HandleMessage(ctx context.Context, e common.Message) error {
	switch evt := e.(type) {
	case message.DeleteLocalUsers:
		for _, username := range evt.Usernames {
			cred := &entity.Credential{
				Source:           preset.AuthSourceLocalUser,
				UserName:         username,
				OrganizationName: preset.DefaultOrganizationName,
			}
			m.Logout(ctx, cred)
		}
	case message.ResetLocalUserPassword:
		cred := &entity.Credential{
			Source:           preset.AuthSourceLocalUser,
			UserName:         evt.Username,
			OrganizationName: preset.DefaultOrganizationName,
		}
		m.Logout(ctx, cred)
	}
	return nil
}
