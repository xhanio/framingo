package repository

import (
	"context"

	"gorm.io/gorm"

	"github.com/xhanio/errors"

	"github.com/xhanio/framingo/example/pkg/types/entity"
	"github.com/xhanio/framingo/example/pkg/types/orm"
	"github.com/xhanio/framingo/example/pkg/types/preset"
)

func (m *manager) CreateOrganization(ctx context.Context, opts entity.OrganizationCreateOptions) (*orm.Organization, error) {
	tx := m.db.FromContext(ctx)
	organization := &orm.Organization{Name: opts.Name}
	if err := tx.Create(&organization).Error; err != nil {
		switch err {
		case gorm.ErrDuplicatedKey:
			return nil, errors.AlreadyExist.Wrapf(err, "organization %s already exists", opts.Name)
		default:
			return nil, errors.DBFailed.Wrap(err)
		}
	}
	return organization, nil
}

func (m *manager) getOrganization(tx *gorm.DB, organizationID int32) (*orm.Organization, error) {
	var organization *orm.Organization
	if err := tx.Model(&organization).Where("id = ?", organizationID).First(&organization).Error; err != nil {
		switch err {
		case gorm.ErrRecordNotFound:
			return nil, errors.NotFound.Wrapf(err, "organization %d not found", organizationID)
		default:
			return nil, errors.DBFailed.Wrap(err)
		}
	}
	return organization, nil
}

func (m *manager) GetOrganization(ctx context.Context, organizationID int32) (*orm.Organization, error) {
	tx := m.db.FromContext(ctx)
	return m.getOrganization(tx, organizationID)
}

func (m *manager) GetOrganizationID(ctx context.Context, organizationName string) (int32, error) {
	if organizationName == preset.DefaultOrganizationName {
		return preset.DefaultOrganizationID, nil
	}
	tx := m.db.FromContext(ctx)
	var org *orm.Organization
	if err := tx.Model(&org).Where("name = ?", organizationName).First(&org).Error; err != nil {
		switch err {
		case gorm.ErrRecordNotFound:
			return 0, errors.NotFound.Wrapf(err, "organization %s not found", organizationName)
		default:
			return 0, errors.DBFailed.Wrap(err)
		}
	}
	return org.ID, nil
}

func (m *manager) UpdateOrganization(ctx context.Context, organizationID int32, opts entity.OrganizationUpdateOptions) (*orm.Organization, error) {
	tx := m.db.FromContext(ctx)
	var organization *orm.Organization
	err := tx.Transaction(func(itx *gorm.DB) error {
		org, err := m.getOrganization(itx, organizationID)
		if err != nil {
			return errors.Wrap(err)
		}
		updates := map[string]any{}
		if opts.Name != nil {
			updates["name"] = *opts.Name
		}
		if len(updates) > 0 {
			if err := itx.Model(&org).Updates(updates).Error; err != nil {
				return errors.DBFailed.Wrapf(err, "failed to update organization %d", organizationID)
			}
		}
		organization = org
		return nil
	})
	if err != nil {
		return nil, errors.Wrapf(err, "failed to update organization %d", organizationID)
	}
	return organization, nil
}

func (m *manager) DeleteOrganization(ctx context.Context, organizationID int32) error {
	tx := m.db.FromContext(ctx)
	err := tx.Transaction(func(itx *gorm.DB) error {
		org, err := m.getOrganization(itx, organizationID)
		if err != nil {
			return errors.Wrap(err)
		}
		if err := itx.Model(&org).Where("id = ?", organizationID).Delete(&org).Error; err != nil {
			return errors.DBFailed.Wrap(err)
		}
		return nil
	})
	if err != nil {
		return errors.Wrapf(err, "failed to delete organization %d", organizationID)
	}
	return nil
}
