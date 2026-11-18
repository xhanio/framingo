package role

import (
	"context"

	"github.com/xhanio/errors"
	"github.com/xhanio/framingo/pkg/utils/sliceutil"

	"github.com/xhanio/framingo/example/pkg/types/entity"
	"github.com/xhanio/framingo/example/pkg/types/rbac"
)

// the admin role holds every permission implicitly, so it is never edited,
// deleted or given a permission list; enforced here so every caller meets it

func (m *manager) Create(ctx context.Context, opts entity.RoleCreateOptions) error {
	if _, err := m.repository.CreateRole(ctx, opts); err != nil {
		return errors.Wrap(err)
	}
	return nil
}

func (m *manager) GetByName(ctx context.Context, roleName string) (*entity.Role, error) {
	role, err := m.repository.GetRoleByName(ctx, roleName)
	if err != nil {
		return nil, errors.Wrap(err)
	}
	return role, nil
}

func (m *manager) Update(ctx context.Context, roleID int32, opts entity.RoleUpdateOptions) error {
	if roleID == rbac.RoleAdminID {
		return errors.BadRequest.Newf("admin role cannot be modified")
	}
	if _, err := m.repository.UpdateRole(ctx, roleID, opts); err != nil {
		return errors.Wrap(err)
	}
	return nil
}

func (m *manager) List(ctx context.Context) ([]*entity.Role, error) {
	roles, err := m.repository.ListRoles(ctx)
	if err != nil {
		return nil, errors.Wrap(err)
	}
	return roles, nil
}

// Delete refuses a role still held by a user; the shared transaction does not
// stop a concurrent assignment.
func (m *manager) Delete(ctx context.Context, roleID int32) error {
	if roleID == rbac.RoleAdminID {
		return errors.BadRequest.Newf("admin role cannot be deleted")
	}
	err := m.repository.Transaction(ctx, func(tctx context.Context) error {
		role, err := m.repository.GetRole(tctx, roleID)
		if err != nil {
			return errors.Wrap(err)
		}
		users, err := m.repository.CountRoleUsers(tctx, role.Name)
		if err != nil {
			return errors.Wrap(err)
		}
		if users > 0 {
			return errors.BadRequest.Newf("Role %s has users assigned to it. Cannot delete it.", role.Name)
		}
		return m.repository.DeleteRole(tctx, roleID)
	})
	if err != nil {
		return errors.Wrap(err)
	}
	return nil
}

func checkPermissions(permissions []string) error {
	for _, p := range permissions {
		if !rbac.IsPermissionValid(p) {
			return errors.BadRequest.Newf("invalid permission: %s", p)
		}
	}
	return nil
}

func (m *manager) SetPermissions(ctx context.Context, roleID int32, opts entity.PermissionSetOptions) error {
	if roleID == rbac.RoleAdminID {
		return errors.BadRequest.Newf("admin role already has all permissions")
	}
	if err := checkPermissions(opts.Permissions); err != nil {
		return errors.Wrap(err)
	}
	if err := m.repository.SetRolePermissions(ctx, roleID, opts.Permissions); err != nil {
		return errors.Wrap(err)
	}
	return nil
}

func (m *manager) SetPermissionsForRoleName(ctx context.Context, roleName string, opts entity.PermissionSetOptions) error {
	if roleName == rbac.RoleAdmin {
		return errors.BadRequest.Newf("admin role already has all permissions")
	}
	if err := checkPermissions(opts.Permissions); err != nil {
		return errors.Wrap(err)
	}
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

// HasPermission returns false for an empty permission; callers treat empty as "none required".
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
