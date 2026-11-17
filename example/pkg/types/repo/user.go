package repo

import (
	"context"

	"github.com/xhanio/framingo/example/pkg/types/entity"
)

// UserCreateOptions is what the repository stores for a new user: the
// password's hash, never the password - how it is hashed is the caller's. It
// says what a user is made of, not how the rows lay it out: the repository
// splits it into a user and a contact row.
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
	// SetUserPasswordHash stores a new hash and clears the password-reset
	// requirement.
	SetUserPasswordHash(ctx context.Context, userID int32, passwordHash string) error
	GetUserByName(ctx context.Context, organizationID int32, username string) (*entity.User, error)
	// GetUserPasswordHash is the stored hash, which entity.User does not
	// carry so that it never reaches a response.
	GetUserPasswordHash(ctx context.Context, userID int32) (string, error)
}
