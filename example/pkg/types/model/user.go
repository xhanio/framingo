package model

import (
	"context"

	"github.com/xhanio/framingo/pkg/types/common"

	"github.com/xhanio/framingo/example/pkg/types/entity"
)

// User manages local user accounts by ID. It does not ask who is calling:
// whether a caller may use an operation is its route's permission, and which
// account is the caller's own the router resolves from the credential
// (Credential.LocalUserID), and that a role given exists the router asks the
// role service. What the service keeps are the rules about an account itself,
// which no caller lifts - the built-in admin is neither deleted nor demoted,
// a password changes only on the old one - and the events the auth service
// ends sessions on.
type User interface {
	common.Service
	Create(ctx context.Context, opts entity.UserCreateOptions) (*entity.User, error)
	List(ctx context.Context, opts entity.UserListOptions) ([]*entity.User, error)
	Delete(ctx context.Context, userIDs []int32) error
	Get(ctx context.Context, userID int32) (*entity.User, error)
	Update(ctx context.Context, userID int32, opts entity.UserUpdateOptions) (*entity.User, error)
	// ResetPassword sets a password without the old one; ChangePassword
	// takes the old one and refuses a wrong one.
	ResetPassword(ctx context.Context, userID int32, opts entity.UserResetPasswordOptions) error
	ChangePassword(ctx context.Context, userID int32, opts entity.UserChangePasswordOptions) error
	// GetSAMLMetadata(ctx context.Context) error
	// UpdateSAMLSettings(ctx context.Context) error
	// DeleteSAMLSettings(ctx context.Context) error
}
