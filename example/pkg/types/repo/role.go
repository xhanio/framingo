package repo

import (
	"context"

	"github.com/xhanio/framingo/example/pkg/types/entity"
)

type Role interface {
	CreateRole(ctx context.Context, opts entity.RoleCreateOptions) (*entity.Role, error)
	GetRole(ctx context.Context, roleID int32) (*entity.Role, error)
	GetRoleByName(ctx context.Context, roleName string) (*entity.Role, error)
	ListRoles(ctx context.Context) ([]*entity.Role, error)
	UpdateRole(ctx context.Context, roleID int32, opts entity.RoleUpdateOptions) (*entity.Role, error)
	// DeleteRole deletes the role and its permissions; whether it may be
	// deleted is the caller's to decide.
	DeleteRole(ctx context.Context, roleID int32) error
	// CountRoleUsers is how many users hold the role.
	CountRoleUsers(ctx context.Context, roleName string) (int64, error)
	AddRolePermission(ctx context.Context, roleID int32, permission string) error
	SetRolePermissions(ctx context.Context, roleID int32, permissions []string) error
	SetRolePermissionsByName(ctx context.Context, roleName string, permissions []string) error
	ListRolePermissions(ctx context.Context, roleID int32) ([]string, error)
	ListRolePermissionsByName(ctx context.Context, roleName string) ([]string, error)
}
