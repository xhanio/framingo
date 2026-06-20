package auth

import (
	"context"
	"io"

	"github.com/xhanio/framingo/pkg/types/common"
	"github.com/xhanio/framingo/pkg/utils/infra"
	"github.com/xhanio/framingo/pkg/utils/printutil"

	"github.com/xhanio/framingo/example/pkg/types/entity"
	"github.com/xhanio/framingo/example/pkg/types/message"
	"github.com/xhanio/framingo/example/pkg/types/preset"
)

func (m *manager) Info(w io.Writer, debug bool) {
	if debug {
		m.RLock()
		defer m.RUnlock()
		t := printutil.NewTable(w)
		t.Header(m.Name())
		t.Title("Credential", "Sessions")
		for cid, sessions := range m.users {
			t.Row(cid, len(sessions))
		}
		t.NewLine()
		t.Title("SessionID", "Credential", "Expired", "ExpiresAt")
		for sid, session := range m.sessions {
			t.Row(sid, session.Credential.UID(), session.Lease.Expired(), session.Lease.ExpiresAt().In(infra.Timezone).Format(common.TimeFormat))
		}
		t.NewLine()
		t.Flush()
	}
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
			m.Logout(context.Background(), cred)
		}
	case message.ResetLocalUserPassword:
		cred := &entity.Credential{
			Source:           preset.AuthSourceLocalUser,
			UserName:         evt.Username,
			OrganizationName: preset.DefaultOrganizationName,
		}
		m.Logout(context.Background(), cred)
	}

	return nil
}
