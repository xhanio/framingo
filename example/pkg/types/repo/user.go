package repo

import (
	"context"

	"github.com/xhanio/framingo/example/pkg/types/entity"
)

// UserCreateOptions carries the password hash, never the password; hashing is the caller's.
type UserCreateOptions struct {
	OrganizationID       int32
	Username             string
	PasswordHash         string
	Role                 string
	FirstName            string
	LastName             string
	Email                string
	Title                string
	ChangePassword       bool
	RequirePasswordReset bool
}

type User interface {
	Repository
	CreateUser(ctx context.Context, opts UserCreateOptions) (*entity.User, error)
	GetUser(ctx context.Context, userID int32) (*entity.User, error)
	ListUsers(ctx context.Context, opts entity.UserListOptions) ([]*entity.User, error)
	UpdateUser(ctx context.Context, userID int32, opts entity.UserUpdateOptions) (*entity.User, error)
	DeleteUser(ctx context.Context, userID int32) error
	DeleteUsers(ctx context.Context, userIDs []int32) error
	// SetUserPasswordHash also clears the password-reset requirement.
	SetUserPasswordHash(ctx context.Context, userID int32, passwordHash string) error
	GetUserByName(ctx context.Context, organizationID int32, username string) (*entity.User, error)
	// GetUserPasswordHash exists because entity.User never carries the hash, keeping it out of responses.
	GetUserPasswordHash(ctx context.Context, userID int32) (string, error)
}
