package repository

import (
	"context"

	"gorm.io/gorm"

	"github.com/xhanio/errors"
	"github.com/xhanio/framingo/pkg/utils/certutil"

	"github.com/xhanio/framingo/example/pkg/types/entity"
	"github.com/xhanio/framingo/example/pkg/types/orm"
	"github.com/xhanio/framingo/example/pkg/types/preset"
)

func (m *manager) CreateCertificate(ctx context.Context, opts entity.CertCreateOptions) (*orm.Certificate, error) {
	bundleBytes, err := certutil.Encode(opts.Bundle)
	if err != nil {
		return nil, errors.Wrap(err)
	}
	cert := &orm.Certificate{
		Name:       opts.Name,
		IsCA:       opts.IsCA,
		IsLocal:    opts.IsLocal,
		Type:       opts.Type,
		CertBundle: bundleBytes,
		Comments:   opts.Comments,
		Source:     opts.Source,
	}
	tx := m.db.FromContext(ctx)
	if err := tx.Create(&cert).Error; err != nil {
		switch err {
		case gorm.ErrDuplicatedKey:
			return nil, errors.AlreadyExist.Wrapf(err, "certificate %s already exists", cert.Name)
		default:
			return nil, errors.DBFailed.Wrap(err)
		}
	}
	return cert, nil
}

func (m *manager) getCertificate(tx *gorm.DB, certID int32) (*orm.Certificate, error) {
	var cert *orm.Certificate
	if err := tx.Model(&orm.Certificate{}).Where("id = ?", certID).First(&cert).Error; err != nil {
		switch err {
		case gorm.ErrRecordNotFound:
			return nil, errors.NotFound.Wrapf(err, "certificate %d not found", certID)
		default:
			return nil, errors.DBFailed.Wrap(err)
		}
	}
	return cert, nil
}

func (m *manager) GetCertificate(ctx context.Context, certID int32) (*orm.Certificate, error) {
	tx := m.db.FromContext(ctx)
	return m.getCertificate(tx, certID)
}

func (m *manager) GetCertificateByName(ctx context.Context, certName string) (*orm.Certificate, error) {
	tx := m.db.FromContext(ctx)
	var cert *orm.Certificate
	if err := tx.Model(&orm.Certificate{}).Where("name = ?", certName).First(&cert).Error; err != nil {
		switch err {
		case gorm.ErrRecordNotFound:
			return nil, errors.NotFound.Wrapf(err, "certificate %s not found", certName)
		default:
			return nil, errors.DBFailed.Wrap(err)
		}
	}
	return cert, nil
}

func (m *manager) ListCertificates(ctx context.Context, opts entity.CertListOptions) ([]*orm.Certificate, error) {
	tx := m.db.FromContext(ctx)
	var certs []*orm.Certificate

	query := tx.Model(&orm.Certificate{})
	if opts.IsCA != nil {
		if *opts.IsCA {
			query = query.Where("is_ca = ?", true)
		} else {
			query = query.Where("is_ca = ?", false)
		}
	}
	if opts.IsLocal {
		query = query.Where("is_local = ?", true)
	}
	// Exclude the default CA
	query = query.Where("name != ?", preset.CAName)

	if err := query.Find(&certs).Error; err != nil {
		return nil, errors.DBFailed.Wrapf(err, "failed to list certificates")
	}
	return certs, nil
}

func (m *manager) UpdateCertificateComments(ctx context.Context, certID int32, comments string) error {
	tx := m.db.FromContext(ctx)
	err := tx.Transaction(func(itx *gorm.DB) error {
		cert, err := m.getCertificate(itx, certID)
		if err != nil {
			return errors.Wrap(err)
		}
		cert.Comments = comments
		if err := itx.Save(&cert).Error; err != nil {
			return errors.DBFailed.Wrapf(err, "failed to update comments for certificate %d", certID)
		}
		return nil
	})
	if err != nil {
		return errors.Wrapf(err, "failed to update comments for certificate %d", certID)
	}
	return nil
}

func (m *manager) IncCertificateRefCount(ctx context.Context, certID int32) error {
	tx := m.db.FromContext(ctx)
	err := tx.Transaction(func(itx *gorm.DB) error {
		cert, err := m.getCertificate(itx, certID)
		if err != nil {
			return errors.Wrap(err)
		}
		newCount := cert.RefCount + 1
		if err := itx.Model(&cert).Where("id = ?", certID).Update("ref_count", newCount).Error; err != nil {
			return errors.DBFailed.Wrapf(err, "failed to increment ref_count for certificate %d", certID)
		}
		return nil
	})
	if err != nil {
		return errors.Wrapf(err, "failed to increment ref_count for certificate %d", certID)
	}
	return nil
}

func (m *manager) DecCertificateRefCount(ctx context.Context, certID int32) error {
	tx := m.db.FromContext(ctx)
	err := tx.Transaction(func(itx *gorm.DB) error {
		cert, err := m.getCertificate(itx, certID)
		if err != nil {
			return errors.Wrap(err)
		}
		if cert.RefCount == 0 {
			return nil
		}
		newCount := cert.RefCount - 1
		if err := itx.Model(&cert).Where("id = ?", certID).Update("ref_count", newCount).Error; err != nil {
			return errors.DBFailed.Wrapf(err, "failed to decrement ref_count for certificate %d", certID)
		}
		return nil
	})
	if err != nil {
		return errors.Wrapf(err, "failed to decrement ref_count for certificate %d", certID)
	}
	return nil
}

func (m *manager) DeleteCertificate(ctx context.Context, certID int32) error {
	tx := m.db.FromContext(ctx)
	if err := tx.Where("id = ?", certID).Delete(&orm.Certificate{}).Error; err != nil {
		return errors.DBFailed.Wrapf(err, "failed to delete certificate %d", certID)
	}
	return nil
}
