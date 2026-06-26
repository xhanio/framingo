package repository

import (
	"context"
	"encoding/json"

	"gorm.io/gorm"

	"github.com/xhanio/errors"

	"github.com/xhanio/framingo/example/pkg/types/entity"
	"github.com/xhanio/framingo/example/pkg/types/orm"
)

func (m *manager) CreateRole(ctx context.Context, opts entity.RoleCreateOptions) (*orm.Role, error) {
	tx := m.db.FromContext(ctx)
	role := &orm.Role{
		Name:        opts.Name,
		Description: opts.Description,
	}
	if err := tx.Create(&role).Error; err != nil {
		switch err {
		case gorm.ErrDuplicatedKey:
			return nil, errors.AlreadyExist.Wrapf(err, "role %s already exists", opts.Name)
		default:
			return nil, errors.DBFailed.Wrap(err)
		}
	}
	return role, nil
}

func (m *manager) getRole(tx *gorm.DB, roleID int32) (*orm.Role, error) {
	var role *orm.Role
	if err := tx.Model(&role).Where("id = ?", roleID).First(&role).Error; err != nil {
		switch err {
		case gorm.ErrRecordNotFound:
			return nil, errors.NotFound.Wrapf(err, "role %d not found", roleID)
		default:
			return nil, errors.DBFailed.Wrap(err)
		}
	}
	return role, nil
}

func (m *manager) GetRole(ctx context.Context, roleID int32) (*orm.Role, error) {
	tx := m.db.FromContext(ctx)
	return m.getRole(tx, roleID)
}

func (m *manager) getRoleByName(tx *gorm.DB, roleName string) (*orm.Role, error) {
	var role *orm.Role
	if err := tx.Model(&role).Where("name = ?", roleName).First(&role).Error; err != nil {
		switch err {
		case gorm.ErrRecordNotFound:
			return nil, errors.NotFound.Wrapf(err, "role %s not found", roleName)
		default:
			return nil, errors.DBFailed.Wrap(err)
		}
	}
	return role, nil
}

func (m *manager) GetRoleByName(ctx context.Context, roleName string) (*orm.Role, error) {
	tx := m.db.FromContext(ctx)
	return m.getRoleByName(tx, roleName)
}

func (m *manager) ListRoles(ctx context.Context) ([]*orm.Role, error) {
	tx := m.db.FromContext(ctx)
	var roles []*orm.Role
	if err := tx.Model(&orm.Role{}).Find(&roles).Error; err != nil {
		return nil, errors.DBFailed.Wrapf(err, "failed to list roles")
	}
	return roles, nil
}

func roleUpdateOptionsToMap(opts entity.RoleUpdateOptions) (map[string]any, error) {
	bytes, err := json.Marshal(opts)
	if err != nil {
		return nil, errors.Wrap(err)
	}
	result := map[string]any{}
	if err := json.Unmarshal(bytes, &result); err != nil {
		return nil, errors.Wrap(err)
	}
	return result, nil
}

func (m *manager) UpdateRole(ctx context.Context, roleID int32, opts entity.RoleUpdateOptions) (*orm.Role, error) {
	tx := m.db.FromContext(ctx)
	var role *orm.Role
	err := tx.Transaction(func(itx *gorm.DB) error {
		r, err := m.getRole(itx, roleID)
		if err != nil {
			return errors.Wrap(err)
		}
		updateMap, err := roleUpdateOptionsToMap(opts)
		if err != nil {
			return errors.Wrap(err)
		}
		if err := itx.Model(&r).Updates(updateMap).Error; err != nil {
			return errors.DBFailed.Wrapf(err, "failed to update role %d", roleID)
		}
		role = r
		return nil
	})
	if err != nil {
		return nil, errors.Wrapf(err, "failed to update role %d", roleID)
	}
	return role, nil
}

func (m *manager) DeleteRole(ctx context.Context, roleID int32) error {
	tx := m.db.FromContext(ctx)
	err := tx.Transaction(func(itx *gorm.DB) error {
		r, err := m.getRole(itx, roleID)
		if err != nil {
			return errors.Wrap(err)
		}
		// Check if the role has any users assigned to it
		var users []*orm.User
		if err := itx.Model(&users).Where("role = ?", r.Name).Find(&users).Error; err != nil {
			return errors.DBFailed.Wrapf(err, "failed to get users for role %s", r.Name)
		}
		if len(users) > 0 {
			return errors.BadRequest.Newf("Role %s has users assigned to it. Cannot delete it.", r.Name)
		}
		// Delete the role
		if err := itx.Model(&r).Where("id = ?", roleID).Delete(&r).Error; err != nil {
			return errors.DBFailed.Wrapf(err, "failed to delete role %d", roleID)
		}
		// Delete the role's permissions
		if err := itx.Where("role_id = ?", roleID).Delete(&orm.RolePermission{}).Error; err != nil {
			return errors.DBFailed.Wrapf(err, "failed to delete role permissions for role %d", roleID)
		}
		return nil
	})
	if err != nil {
		return errors.Wrapf(err, "failed to delete role %d", roleID)
	}
	return nil
}

func (m *manager) AddRolePermission(ctx context.Context, roleID int32, permission string) error {
	tx := m.db.FromContext(ctx)
	err := tx.Transaction(func(itx *gorm.DB) error {
		if _, err := m.getRole(itx, roleID); err != nil {
			return errors.Wrap(err)
		}
		rp := &orm.RolePermission{
			RoleID:     roleID,
			Permission: permission,
		}
		if err := itx.Create(&rp).Error; err != nil {
			return errors.DBFailed.Wrapf(err, "failed to create permission for role %d", roleID)
		}
		return nil
	})
	if err != nil {
		return errors.Wrapf(err, "failed to add permission for role %d", roleID)
	}
	return nil
}

func (m *manager) setRolePermissions(tx *gorm.DB, roleID int32, permissions []string) error {
	// Delete existing permissions
	if err := tx.Where("role_id = ?", roleID).Delete(&orm.RolePermission{}).Error; err != nil {
		return errors.DBFailed.Wrapf(err, "failed to delete role permissions for role %d", roleID)
	}
	if len(permissions) == 0 {
		return nil
	}
	var rolePermissions []*orm.RolePermission
	for i := range permissions {
		rolePermissions = append(rolePermissions, &orm.RolePermission{
			RoleID:     roleID,
			Permission: permissions[i],
		})
	}
	if err := tx.Create(&rolePermissions).Error; err != nil {
		return errors.DBFailed.Wrapf(err, "failed to set permissions for role %d", roleID)
	}
	return nil
}

func (m *manager) SetRolePermissions(ctx context.Context, roleID int32, permissions []string) error {
	tx := m.db.FromContext(ctx)
	err := tx.Transaction(func(itx *gorm.DB) error {
		if _, err := m.getRole(itx, roleID); err != nil {
			return errors.Wrap(err)
		}
		return m.setRolePermissions(itx, roleID, permissions)
	})
	if err != nil {
		return errors.Wrapf(err, "failed to set permissions for role %d", roleID)
	}
	return nil
}

func (m *manager) SetRolePermissionsByName(ctx context.Context, roleName string, permissions []string) error {
	tx := m.db.FromContext(ctx)
	err := tx.Transaction(func(itx *gorm.DB) error {
		role, err := m.getRoleByName(itx, roleName)
		if err != nil {
			return errors.Wrap(err)
		}
		return m.setRolePermissions(itx, role.ID, permissions)
	})
	if err != nil {
		return errors.Wrapf(err, "failed to set permissions for role %s", roleName)
	}
	return nil
}

func (m *manager) ListRolePermissions(ctx context.Context, roleID int32) ([]string, error) {
	tx := m.db.FromContext(ctx)
	var permissions []orm.RolePermission
	err := tx.Transaction(func(itx *gorm.DB) error {
		if _, err := m.getRole(itx, roleID); err != nil {
			return errors.Wrap(err)
		}
		if err := itx.Model(&orm.RolePermission{}).Where("role_id = ?", roleID).Find(&permissions).Error; err != nil {
			return errors.DBFailed.Wrap(err)
		}
		return nil
	})
	if err != nil {
		return nil, errors.Wrapf(err, "failed to list permissions for role %d", roleID)
	}
	result := make([]string, len(permissions))
	for i, p := range permissions {
		result[i] = p.Permission
	}
	return result, nil
}

func (m *manager) ListRolePermissionsByName(ctx context.Context, roleName string) ([]string, error) {
	tx := m.db.FromContext(ctx)
	var permissions []orm.RolePermission
	if err := tx.Model(&orm.RolePermission{}).
		Joins("JOIN roles ON role_permissions.role_id = roles.id").
		Where("roles.name = ?", roleName).
		Find(&permissions).Error; err != nil {
		return nil, errors.DBFailed.Wrapf(err, "failed to list permissions for role %s", roleName)
	}
	result := make([]string, len(permissions))
	for i, p := range permissions {
		result[i] = p.Permission
	}
	return result, nil
}
