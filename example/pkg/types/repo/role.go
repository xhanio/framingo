package repo

import (
	"context"

	"github.com/xhanio/framingo/example/pkg/types/entity"
	"github.com/xhanio/framingo/example/pkg/types/orm"
)

type Role interface {
	CreateRole(ctx context.Context, opts entity.RoleCreateOptions) (*orm.Role, error)
	GetRole(ctx context.Context, roleID int32) (*orm.Role, error)
	GetRoleByName(ctx context.Context, roleName string) (*orm.Role, error)
	ListRoles(ctx context.Context) ([]*orm.Role, error)
	UpdateRole(ctx context.Context, roleID int32, opts entity.RoleUpdateOptions) (*orm.Role, error)
	DeleteRole(ctx context.Context, roleID int32) error
	AddRolePermission(ctx context.Context, roleID int32, permission string) error
	SetRolePermissions(ctx context.Context, roleID int32, permissions []string) error
	SetRolePermissionsByName(ctx context.Context, roleName string, permissions []string) error
	ListRolePermissions(ctx context.Context, roleID int32) ([]string, error)
	ListRolePermissionsByName(ctx context.Context, roleName string) ([]string, error)
}
