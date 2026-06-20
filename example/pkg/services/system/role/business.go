package role

import (
	"context"
	"fmt"
	"regexp"
	"strings"

	"github.com/xhanio/errors"

	"github.com/xhanio/framingo/example/pkg/types/entity"
	"github.com/xhanio/framingo/example/pkg/types/rbac"
)

// Register the required permissions for given router handler into the Role Manager.
func (m *manager) RegisterHandlerPermission(action string, path string, permission string) error {
	params := regexp.MustCompile(`:[^/]+`)
	resource := fmt.Sprintf("^%s$", strings.TrimSuffix(params.ReplaceAllString(path, "[^/]+?"), "?"))
	hp := handlerPermission{
		Action:     action,
		Resource:   resource,
		Permission: permission,
	}
	m.handlerInfo = append(m.handlerInfo, hp)
	return nil
}

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

func (m *manager) CheckPermissionByName(ctx context.Context, roleName string, action string, resource string) (bool, error) {
	// Grant all permissions for "admin" role
	if roleName == rbac.RoleAdmin {
		return true, nil
	}

	// Get the current user's role permissions
	permissions, err := m.GetPermissionsByName(ctx, roleName)
	if err != nil {
		return false, errors.Forbidden.Wrap(err)
	}
	if len(permissions) == 0 {
		return false, errors.Forbidden.Newf("No permission is set for role roleName %s", roleName)
	}

	// Get the required permission for the requested action and resource
	var requiredPermission string
	for _, hp := range m.handlerInfo {
		if hp.Action == action && regexp.MustCompile(hp.Resource).MatchString(resource) {
			requiredPermission = hp.Permission
			break
		}
	}

	// Check if the user's role permissions contains the required permission
	for _, s := range permissions {
		if s == requiredPermission {
			return true, nil
		}
	}

	return false, nil
}
