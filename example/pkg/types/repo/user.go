package repo

import (
	"context"

	"github.com/xhanio/framingo/example/pkg/types/entity"
)

type User interface {
	// CreateUser stores passwordHash as the user's password. How a password is
	// hashed is the caller's: the repository never reads opts.Password.
	CreateUser(ctx context.Context, opts entity.UserCreateOptions, passwordHash string) (*entity.User, error)
	GetUser(ctx context.Context, userID int32) (*entity.User, error)
	ListUsers(ctx context.Context, opts entity.UserListOptions) ([]*entity.User, error)
	UpdateUser(ctx context.Context, userID int32, opts entity.UserUpdateOptions) (*entity.User, error)
	DeleteUser(ctx context.Context, userID int32) error
	DeleteUsers(ctx context.Context, userIDs []int32) error
	// SetUserPasswordHash stores a new hash and clears the password-reset
	// requirement.
	SetUserPasswordHash(ctx context.Context, userID int32, passwordHash string) error
	GetUserByName(ctx context.Context, organizationID int32, username string) (*entity.User, error)
	// GetUserPasswordHash is the stored hash, which entity.User does not
	// carry so that it never reaches a response.
	GetUserPasswordHash(ctx context.Context, userID int32) (string, error)
}
