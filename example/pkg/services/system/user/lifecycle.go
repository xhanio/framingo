package user

import (
	"context"

	"github.com/xhanio/errors"

	"github.com/xhanio/framingo/example/pkg/types/entity"
	"github.com/xhanio/framingo/example/pkg/types/preset"
	"github.com/xhanio/framingo/example/pkg/types/rbac"
)

func (m *manager) Init(ctx context.Context) error {
	if _, err := m.repository.GetUserByName(ctx, preset.DefaultOrganizationID, preset.AdminUsername); err == nil {
		return nil
	} else if !errors.Is(err, errors.NotFound) {
		return errors.Wrap(err)
	}
	if _, _, err := m.repository.CreateUser(ctx, entity.UserCreateOptions{
		OrganizationID:       preset.DefaultOrganizationID,
		Username:             preset.AdminUsername,
		Password:             preset.AdminPassword,
		Role:                 rbac.RoleAdmin,
		RequirePasswordReset: true,
	}); err != nil {
		return errors.Wrap(err)
	}
	return nil
}
