package role

import (
	"context"

	"github.com/xhanio/errors"

	"github.com/xhanio/framingo/example/pkg/types/entity"
	"github.com/xhanio/framingo/example/pkg/types/rbac"
)

func (m *manager) Init(ctx context.Context) error {
	defaults := []struct {
		name        string
		permissions []string
	}{
		{rbac.RoleAdmin, rbac.PermissionsAll},
		{rbac.RoleUser, rbac.PermissionsUser},
		{rbac.RoleReadonly, rbac.PermissionsReadonly},
	}
	for _, d := range defaults {
		if _, err := m.repository.GetRoleByName(ctx, d.name); err == nil {
			continue
		} else if !errors.Is(err, errors.NotFound) {
			return errors.Wrap(err)
		}
		if _, err := m.repository.CreateRole(ctx, entity.RoleCreateOptions{Name: d.name}); err != nil {
			return errors.Wrap(err)
		}
		if d.permissions != nil {
			if err := m.repository.SetRolePermissionsByName(ctx, d.name, d.permissions); err != nil {
				return errors.Wrap(err)
			}
		}
	}
	return nil
}
