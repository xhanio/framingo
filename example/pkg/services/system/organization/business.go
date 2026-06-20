package organization

import (
	"context"

	"github.com/xhanio/errors"

	"github.com/xhanio/framingo/example/pkg/types/entity"
)

func (m *manager) Create(ctx context.Context, opts entity.OrganizationCreateOptions) error {
	if _, err := m.repository.CreateOrganization(ctx, opts); err != nil {
		return errors.Wrap(err)
	}
	return nil
}

func (m *manager) Get(ctx context.Context, organizationID int32) (*entity.Organization, error) {
	organization, err := m.repository.GetOrganization(ctx, organizationID)
	if err != nil {
		return nil, errors.Wrap(err)
	}
	return toEntity(organization), nil
}

func (m *manager) Update(ctx context.Context, organizationID int32, opts entity.OrganizationUpdateOptions) error {
	if _, err := m.repository.UpdateOrganization(ctx, organizationID, opts); err != nil {
		return errors.Wrap(err)
	}
	return nil
}

func (m *manager) Delete(ctx context.Context, organizationID int32) error {
	if err := m.repository.DeleteOrganization(ctx, organizationID); err != nil {
		return errors.Wrap(err)
	}
	return nil
}
