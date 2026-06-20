package certificate

import (
	"context"
	"path/filepath"

	"go.step.sm/crypto/x509util"

	"github.com/xhanio/errors"
	"github.com/xhanio/framingo/pkg/types/info"
	"github.com/xhanio/framingo/pkg/utils/certutil"

	"github.com/xhanio/framingo/example/pkg/types/entity"
	"github.com/xhanio/framingo/example/pkg/types/preset"
	"github.com/xhanio/framingo/example/pkg/utils/infra"
)

// Import a certificate
func (m *manager) Import(ctx context.Context, opts entity.CertCreateOptions) (*entity.Certificate, error) {
	if opts.Source == "" {
		opts.Source = entity.CertSourceUser // Default source is user
	}
	if opts.Source != entity.CertSourceUser && opts.Source != entity.CertSourceFactory {
		return nil, errors.InvalidArgument.Newf("Invalid source %s", opts.Source)
	}
	ormCertificate, err := m.repository.CreateCertificate(ctx, opts)
	if err != nil {
		return nil, errors.Wrap(err)
	}
	certificate, err := toEntity(ormCertificate)
	if err != nil {
		return nil, errors.Wrap(err)
	}
	return certificate, nil
}

// Issue a certificate signed by a CA
func (m *manager) Issue(ctx context.Context, opts entity.CertIssueOptions) (*entity.Certificate, error) {
	// Get the CA certificate
	ormCA, err := m.repository.GetCertificate(ctx, opts.CAID)
	if err != nil {
		return nil, errors.Wrapf(err, "Failed to get the CA certificate %d", opts.CAID)
	}

	// Decode the CA certificate
	caCert, err := certutil.Decode(ormCA.CertBundle)
	if err != nil {
		return nil, errors.Wrap(err)
	}
	if !caCert.IsCA() {
		return nil, errors.BadRequest.Newf("The CA certificate %d is not a CA", opts.CAID)
	}
	caBundle := caCert.(certutil.CABundle)

	// Parse common certName and subject alt certName to see if they are IP or domain certName
	dns, ips, _, _ := x509util.SplitSANs([]string{
		opts.CommonName, opts.SubjectAltName,
	})

	serverBundle, err := caBundle.SignServer(&certutil.ServerRequest{
		CommonName: opts.CommonName,
		DNSNames:   dns,
		IPs:        ips,
	})
	if err != nil {
		return nil, errors.Wrap(err)
	}

	// Import the signed cert
	importOpts := entity.CertCreateOptions{
		Name:    opts.Name,
		IsCA:    false,
		IsLocal: true,
		Type:    entity.CertTypeCertKey, // Generated certificate default type
		Bundle:  serverBundle,
	}

	cert, err := m.Import(ctx, importOpts)
	if err != nil {
		return nil, errors.Wrap(err)
	}
	return cert, nil
}

// Get the certificate with all parsed details
func (m *manager) Get(ctx context.Context, certID int32) (*entity.Certificate, error) {
	ormCert, err := m.repository.GetCertificate(ctx, certID)
	if err != nil {
		return nil, errors.Wrap(err)
	}
	entityCert, err := toEntity(ormCert)
	if err != nil {
		return nil, errors.Wrap(err)
	}
	return entityCert, nil
}

func (m *manager) List(ctx context.Context, opts entity.CertListOptions) ([]*entity.Certificate, error) {
	ormCerts, err := m.repository.ListCertificates(ctx, opts)
	if err != nil {
		return nil, errors.Wrap(err)
	}

	var entityCerts []*entity.Certificate
	for _, ormCert := range ormCerts {
		entityCert, err := toEntity(ormCert)
		if err != nil {
			return nil, errors.Wrapf(err, "Failed to extract cert details for cert %s", ormCert.Name)
		}
		entityCerts = append(entityCerts, entityCert)
	}
	return entityCerts, nil
}

func (m *manager) Delete(ctx context.Context, certIDs []int32) error {
	err := m.repository.Transaction(ctx, func(_ctx context.Context) error {
		for _, certID := range certIDs {
			// Check if the certificate exists or not
			cert, err := m.repository.GetCertificate(_ctx, certID)
			if err != nil {
				return errors.Wrap(err)
			}
			// Cannot delete the default CA
			if cert.Name == preset.CAName {
				return errors.BadRequest.Newf("Default CA cannot be deleted")
			}
			// Check if the certificate is being referenced
			if cert.RefCount > 0 {
				return errors.Conflict.Newf("The certificate %s cannot be deleted as it is being referenced", cert.Name)
			}
			// Delete the target certificate
			if err := m.repository.DeleteCertificate(_ctx, certID); err != nil {
				return errors.Wrap(err)
			}
		}
		return nil
	})
	return errors.Wrap(err)
}

func (m *manager) Update(ctx context.Context, certID int32, opts entity.CertUpdateOptions) error {
	if opts.Comments == "" {
		return errors.BadRequest.Newf("Updated comments cannot be empty")
	}
	if err := m.repository.UpdateCertificateComments(ctx, certID, opts.Comments); err != nil {
		return errors.Wrap(err)
	}
	return nil
}

func (m *manager) IncRefCount(ctx context.Context, certID int32) error {
	if err := m.repository.IncCertificateRefCount(ctx, certID); err != nil {
		return errors.Wrap(err)
	}
	return nil
}

func (m *manager) DecRefCount(ctx context.Context, certID int32) error {
	if err := m.repository.DecCertificateRefCount(ctx, certID); err != nil {
		if errors.Is(err, errors.NotFound) {
			m.log.Warnf("Failed to decrease refCount of target cert %d: %s", certID, err)
			return nil
		}
		return errors.Wrap(err)
	}
	return nil
}

func (m *manager) DefaultCA() (certutil.CABundle, error) {
	if m.ca == nil {
		ca, err := m.defaultCA(context.Background())
		if err != nil {
			return nil, errors.Wrap(err)
		}
		err = ca.Dump(filepath.Join(infra.ConfigDir, "cert/ca.crt"), filepath.Join(infra.ConfigDir, "cert/ca.key"))
		if err != nil {
			return nil, errors.Wrap(err)
		}
		m.ca = ca
	}
	return m.ca, nil
}

// Get the default CA from the database, if not found, generate it
func (m *manager) defaultCA(ctx context.Context) (certutil.CABundle, error) {
	ormDefaultCA, err := m.repository.GetCertificateByName(ctx, preset.CAName)
	if err != nil {
		if !errors.Is(err, errors.NotFound) {
			return nil, errors.Wrap(err)
		}
		// Generate the default CA bundle
		cm, err := certutil.New(
			certutil.WithCommonName(info.ProductName),
		)
		if err != nil {
			return nil, errors.Wrap(err)
		}
		if _, err := m.repository.CreateCertificate(ctx, entity.CertCreateOptions{
			Name:    preset.CAName,
			IsCA:    true,
			IsLocal: true,
			Type:    entity.CertTypeCertKey,
			Bundle:  cm,
			Source:  entity.CertSourceFactory,
		}); err != nil {
			return nil, errors.Wrapf(err, "Failed to upload the defaultCA certificate")
		}
		return cm, nil
	}
	// Decode the default CA certificate bundle to get the CA bundle
	caCert, err := certutil.Decode(ormDefaultCA.CertBundle)
	if err != nil {
		return nil, errors.Wrap(err)
	}
	return caCert.(certutil.CABundle), nil
}
