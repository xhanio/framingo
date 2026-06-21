package model

import (
	"context"

	"github.com/xhanio/framingo/pkg/types/common"

	"github.com/xhanio/framingo/example/pkg/types/entity"
)

type Role interface {
	common.Service
	Create(ctx context.Context, opts entity.RoleCreateOptions) error
	Update(ctx context.Context, roleID int32, opts entity.RoleUpdateOptions) error
	List(ctx context.Context) ([]*entity.Role, error)
	Delete(ctx context.Context, roleID int32) error
	GetByName(ctx context.Context, roleName string) (*entity.Role, error)
	SetPermissions(ctx context.Context, roleID int32, opts entity.PermissionSetOptions) error
	GetPermissions(ctx context.Context, roleID int32) ([]string, error)
	GetPermissionsByName(ctx context.Context, roleName string) ([]string, error)
	HasPermission(ctx context.Context, roleName string, permission string) (bool, error)
}
