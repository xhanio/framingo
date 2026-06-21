package role

import (
	"context"

	"github.com/xhanio/errors"

	"github.com/xhanio/framingo/example/pkg/types/entity"
	"github.com/xhanio/framingo/example/pkg/types/rbac"
	"github.com/xhanio/framingo/pkg/utils/sliceutil"
)

func (m *manager) Create(ctx context.Context, opts entity.RoleCreateOptions) error {
	if _, err := m.repository.CreateRole(ctx, opts); err != nil {
		return errors.Wrap(err)
	}
	return nil
}

func (m *manager) GetByName(ctx context.Context, roleName string) (*entity.Role, error) {
	ormRole, err := m.repository.GetRoleByName(ctx, roleName)
	if err != nil {
		return nil, errors.Wrap(err)
	}
	return &entity.Role{
		ID:          ormRole.ID,
		Name:        ormRole.Name,
		Description: ormRole.Description,
	}, nil
}

func (m *manager) Update(ctx context.Context, roleID int32, opts entity.RoleUpdateOptions) error {
	if _, err := m.repository.UpdateRole(ctx, roleID, opts); err != nil {
		return errors.Wrap(err)
	}
	return nil
}

func (m *manager) List(ctx context.Context) ([]*entity.Role, error) {
	ormRoles, err := m.repository.ListRoles(ctx)
	if err != nil {
		return nil, errors.Wrap(err)
	}
	var roles []*entity.Role
	for _, ormRole := range ormRoles {
		roles = append(roles, &entity.Role{
			ID:          ormRole.ID,
			Name:        ormRole.Name,
			Description: ormRole.Description,
		})
	}
	return roles, nil
}

func (m *manager) Delete(ctx context.Context, roleID int32) error {
	if err := m.repository.DeleteRole(ctx, roleID); err != nil {
		return errors.Wrap(err)
	}
	return nil
}

func (m *manager) SetPermissions(ctx context.Context, roleID int32, opts entity.PermissionSetOptions) error {
	if err := m.repository.SetRolePermissions(ctx, roleID, opts.Permissions); err != nil {
		return errors.Wrap(err)
	}
	return nil
}

func (m *manager) SetPermissionsForRoleName(ctx context.Context, roleName string, opts entity.PermissionSetOptions) error {
	if err := m.repository.SetRolePermissionsByName(ctx, roleName, opts.Permissions); err != nil {
		return errors.Wrap(err)
	}
	return nil
}

func (m *manager) GetPermissions(ctx context.Context, roleID int32) ([]string, error) {
	if roleID == rbac.RoleAdminID {
		return rbac.PermissionsAll, nil
	}
	permissions, err := m.repository.ListRolePermissions(ctx, roleID)
	if err != nil {
		return nil, errors.Wrap(err)
	}
	return permissions, nil
}

func (m *manager) GetPermissionsByName(ctx context.Context, roleName string) ([]string, error) {
	if roleName == rbac.RoleAdmin {
		return rbac.PermissionsAll, nil
	}
	permissions, err := m.repository.ListRolePermissionsByName(ctx, roleName)
	if err != nil {
		return nil, errors.Wrap(err)
	}
	return permissions, nil
}

// HasPermission reports whether the role identified by roleName holds the
// given permission. The admin role implicitly holds every permission. An
// empty permission string is treated as "no permission required" by callers
// (e.g., the authz middleware skips the check entirely) and returns false
// here as a defensive default.
func (m *manager) HasPermission(ctx context.Context, roleName string, permission string) (bool, error) {
	if roleName == rbac.RoleAdmin {
		return true, nil
	}
	if permission == "" {
		return false, nil
	}
	permissions, err := m.GetPermissionsByName(ctx, roleName)
	if err != nil {
		return false, errors.Forbidden.Wrap(err)
	}
	return sliceutil.In(permission, permissions...), nil
}
