package repo

import (
	"context"

	"github.com/xhanio/framingo/example/pkg/types/entity"
	"github.com/xhanio/framingo/example/pkg/types/orm"
)

type User interface {
	CreateUser(ctx context.Context, opts entity.UserCreateOptions) (*orm.User, *orm.Contact, error)
	GetUser(ctx context.Context, userID int32) (*orm.User, *orm.Contact, error)
	ListUsers(ctx context.Context, opts entity.UserListOptions) ([]*orm.User, []*orm.Contact, error)
	UpdateUser(ctx context.Context, userID int32, opts entity.UserUpdateOptions) (*orm.User, *orm.Contact, error)
	DeleteUser(ctx context.Context, userID int32) error
	DeleteUsers(ctx context.Context, userIDs []int32) error
	ResetUserPassword(ctx context.Context, userID int32, plainPassword string) error
	GetUserByName(ctx context.Context, organizationID int32, username string) (*orm.User, error)
	GetContact(ctx context.Context, contactID int32) (*orm.Contact, error)
}
