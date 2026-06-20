package auth

import (
	"context"

	"github.com/google/uuid"

	"github.com/xhanio/errors"
	"github.com/xhanio/framingo/pkg/structs/lease"

	"github.com/xhanio/framingo/example/pkg/types/entity"
	"github.com/xhanio/framingo/example/pkg/types/preset"
)

func (m *manager) List(opts entity.AuthListOptions) []*entity.Credential {
	m.RLock()
	defer m.RUnlock()
	var result []*entity.Credential
	for _, session := range m.sessions {
		cred := session.Credential
		matchFilter := opts.Metadata == nil || opts.Metadata.Empty() || opts.Metadata.Matches(cred.Metadata)
		matchSource := opts.Source == "" || opts.Source == cred.Source
		matchRole := opts.Role == "" || opts.Role == cred.Role
		if matchFilter && matchSource && matchRole {
			result = append(result, cred)
		}
	}
	return result
}

func (m *manager) Login(ctx context.Context, organization, username, password string) (*entity.Session, error) {
	cred, err := m.AuthenticateUser(ctx, organization, username, password)
	if err != nil {
		return nil, errors.Unauthorized.Wrap(err)
	}
	m.Lock()
	defer m.Unlock()
	session := m.createSession(ctx, cred)
	return session, nil
}

// The system attempts authentication using the LDAP method first.
// If the LDAP authentication fails or is not enabled, then it attempts local user authentication
func (m *manager) AuthenticateUser(ctx context.Context, organization, username, password string) (*entity.Credential, error) {
	cred, ldapErr := m.authenticateLDAPUser(ctx, username, password)
	if ldapErr == nil {
		return cred, nil
	}

	cred, localUserErr := m.authenticateLocalUser(ctx, organization, username, password)
	if localUserErr == nil {
		return cred, nil
	} else {
		// If both LDAP and local user authentication fail, return the LDAP error
		if baseErr, ok := ldapErr.(errors.Error); ok {
			if baseErr.Category() == errors.Forbidden {
				return nil, errors.Wrap(ldapErr)
			}
		}
	}

	return nil, errors.Unauthorized.Newf("failed to authenticate user %s", username)

}

func (m *manager) authenticateLDAPUser(ctx context.Context, username, password string) (*entity.Credential, error) {
	if m.lm == nil {
		return nil, errors.NotImplemented.Newf("ladp authenticator not implemented")
	}
	cred, err := m.lm.Authenticate(ctx, username, password)
	if err != nil {
		return nil, errors.Wrapf(err, "failed to authenticate user %s", username)
	}
	if cred == nil {
		return nil, errors.Unauthorized.Newf("ladp user not found")
	}
	return cred, nil
}

func (m *manager) authenticateLocalUser(ctx context.Context, organization, username, password string) (*entity.Credential, error) {
	if m.um == nil {
		return nil, errors.NotImplemented.Newf("user authenticator not implemented")
	}
	cred, err := m.um.Authenticate(ctx, organization, username, password)
	if err != nil {
		return nil, errors.Wrap(err)
	}
	if cred == nil {
		return nil, errors.Unauthorized.Newf("local user not found")
	}
	return cred, nil
}

func (m *manager) AuthenticateAPIToken(ctx context.Context, token string) (*entity.Credential, error) {
	if m.tm == nil {
		return nil, errors.NotImplemented.Newf("token authenticator not implemented")
	}
	cred, err := m.tm.Authenticate(ctx, token)
	if err != nil {
		return nil, errors.Wrap(err)
	}
	if cred == nil {
		return nil, errors.Unauthorized.Newf("api token not found")
	}
	return cred, nil
}

func (m *manager) createSession(ctx context.Context, credential *entity.Credential) *entity.Session {
	id := uuid.NewString()
	l := lease.New(id, preset.SessionExpiration, lease.OnExpired(func() {
		m.Lock()
		sessions, ok := m.users[credential.UID()]
		if ok {
			delete(sessions, id)
		}
		delete(m.sessions, id)
		m.Unlock()
	}), lease.OnCancel(func() {
		m.Lock()
		sessions, ok := m.users[credential.UID()]
		if ok {
			delete(sessions, id)
		}
		delete(m.sessions, id)
		m.Unlock()
	}), lease.WithLogger(m.log))
	session := &entity.Session{
		ID:         id,
		Credential: credential,
		Lease:      l,
	}
	m.sessions[id] = session
	sessions, ok := m.users[credential.UID()]
	if !ok {
		sessions = make(map[string]any)
		m.users[credential.UID()] = sessions
	}
	sessions[session.ID] = nil
	go session.Lease.Start()
	return session
}

func (m *manager) refreshSession(ctx context.Context, sessionID string) (*entity.Session, bool) {
	session, ok := m.sessions[sessionID]
	if ok {
		session.Lease.Refresh(preset.SessionExpiration)
	}
	return session, ok
}

func (m *manager) closeSession(ctx context.Context, sessionID string) bool {
	session, ok := m.sessions[sessionID]
	if ok {
		session.Lease.Cancel()
	}
	return ok
}

func (m *manager) GetSession(ctx context.Context, sessionID string) (*entity.Session, bool) {
	m.RLock()
	defer m.RUnlock()
	session, ok := m.sessions[sessionID]
	return session, ok
}

func (m *manager) HasSession(ctx context.Context, credential *entity.Credential) bool {
	m.RLock()
	defer m.RUnlock()
	sessions := m.users[credential.UID()]
	return len(sessions) > 0
}

func (m *manager) RefreshSession(ctx context.Context, sessionID string) bool {
	m.RLock()
	defer m.RUnlock()
	_, ok := m.refreshSession(ctx, sessionID)
	return ok
}

func (m *manager) CloseSession(ctx context.Context, sessionID string) bool {
	m.Lock()
	defer m.Unlock()
	return m.closeSession(ctx, sessionID)
}

func (m *manager) Logout(ctx context.Context, credential *entity.Credential) bool {
	m.Lock()
	defer m.Unlock()
	result := false
	if sessions, ok := m.users[credential.UID()]; ok {
		for sessionID := range sessions {
			result = m.closeSession(ctx, sessionID) || result
		}
		delete(m.users, credential.UID())
	}
	return result
}

// // PAM related
// func (m *manager) Validate(ctx context.Context, organization, username string) (bool, error) {
// 	ok, err := m.um.Validate(ctx, organization, username)
// 	if err != nil {
// 		return false, errors.Wrap(err)
// 	}
// 	return ok, nil
// }

// // PAM related
// func (m *manager) OpenPAMSession(ctx context.Context, organization, username, password string) (*entity.Session, error) {
// 	return m.Login(ctx, organization, username, password)
// }

// // PAM related
// func (m *manager) ClosePAMSession(ctx context.Context, organization, username, password string) error {
// 	credential, err := m.AuthenticateUser(ctx, organization, username, password)
// 	if err != nil {
// 		return errors.Unauthorized
// 	}
// 	m.Logout(ctx, credential)
// 	return nil
// }
