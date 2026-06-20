package model

import (
	"context"

	"github.com/xhanio/framingo/pkg/types/common"

	"github.com/xhanio/framingo/example/pkg/types/entity"
)

type UserAuthN interface {
	common.Service
	Validate(ctx context.Context, organization, username string) (bool, error)
	Authenticate(ctx context.Context, organization, username, password string) (*entity.Credential, error)
}

type LDAPAuthN interface {
	common.Service
	Authenticate(ctx context.Context, dn, password string) (*entity.Credential, error)
}

type APITokenAuthN interface {
	common.Service
	Authenticate(ctx context.Context, token string) (*entity.Credential, error)
}

type Auth interface {
	common.Service
	Login(ctx context.Context, organization, username, password string) (*entity.Session, error)
	Logout(ctx context.Context, credential *entity.Credential) bool
	GetSession(ctx context.Context, sessionID string) (*entity.Session, bool)
	HasSession(ctx context.Context, credential *entity.Credential) bool
	RefreshSession(ctx context.Context, sessionID string) bool
	CloseSession(ctx context.Context, sessionID string) bool
	AuthenticateUser(ctx context.Context, organization, username, password string) (*entity.Credential, error)
	AuthenticateAPIToken(ctx context.Context, token string) (*entity.Credential, error)
	// the following methods are PAM related
	// Validate(ctx context.Context, organization, username string) (bool, error)
	// OpenSession(ctx context.Context, organization, username, password string) (*entity.Session, error)
	List(opts entity.AuthListOptions) []*entity.Credential
}
