package certificate

import (
	"context"
	"crypto/x509"
	"strings"
	"time"

	"github.com/xhanio/errors"
	"go.step.sm/crypto/x509util"

	"github.com/xhanio/framingo/pkg/types/info"
	"github.com/xhanio/framingo/pkg/utils/certutil"

	"github.com/xhanio/framingo/example/pkg/types/entity"
	"github.com/xhanio/framingo/example/pkg/types/preset"
)

func (m *manager) Import(ctx context.Context, opts entity.CertCreateOptions) (*entity.Certificate, error) {
	if opts.Source == "" {
		opts.Source = entity.CertSourceUser
	}
	if opts.Source != entity.CertSourceUser && opts.Source != entity.CertSourceFactory {
		return nil, errors.InvalidArgument.Newf("Invalid source %s", opts.Source)
	}
	certificate, err := m.repository.CreateCertificate(ctx, opts)
	if err != nil {
		return nil, errors.Wrap(err)
	}
	return describe(certificate)
}

// Issue signs with the CA opts.CAID names, the server CA when it names none.
func (m *manager) Issue(ctx context.Context, opts entity.CertIssueOptions) (*entity.Certificate, error) {
	caBundle, err := m.ServerCA()
	if err != nil {
		return nil, errors.Wrap(err)
	}
	if opts.CAID != 0 {
		ca, err := m.repository.GetCertificate(ctx, opts.CAID)
		if err != nil {
			return nil, errors.Wrapf(err, "Failed to get the CA certificate %d", opts.CAID)
		}
		named, ok := ca.Bundle.(certutil.CABundle)
		if !ok || !ca.Bundle.IsCA() {
			return nil, errors.BadRequest.Newf("The CA certificate %d is not a CA", opts.CAID)
		}
		caBundle = named
	}

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

	importOpts := entity.CertCreateOptions{
		Name:    opts.Name,
		IsCA:    false,
		IsLocal: true,
		Type:    entity.CertTypeCertKey,
		Bundle:  serverBundle,
	}

	cert, err := m.Import(ctx, importOpts)
	if err != nil {
		return nil, errors.Wrap(err)
	}
	return cert, nil
}

// defaultSignValidity is short because a requester renews rather than holds a
// certificate for the CA's lifetime.
const defaultSignValidity = 90 * 24 * time.Hour

// SignCSR signs with the agent CA, which agents are verified against, and
// stores only the certificate: the private key never reaches the manager.
func (m *manager) SignCSR(ctx context.Context, opts entity.CertSignOptions) (*entity.Certificate, error) {
	usage := x509.ExtKeyUsageClientAuth
	switch opts.Usage {
	case "", entity.CertUsageClient:
	case entity.CertUsageServer:
		usage = x509.ExtKeyUsageServerAuth
	default:
		return nil, errors.InvalidArgument.Newf("Unknown certificate usage %s", opts.Usage)
	}
	if opts.ValidPeriod <= 0 {
		opts.ValidPeriod = defaultSignValidity
	}
	csr, err := certutil.ParseCSR(opts.CSR)
	if err != nil {
		return nil, errors.Wrap(err)
	}
	ca, err := m.AgentCA()
	if err != nil {
		return nil, errors.Wrap(err)
	}
	dns, ips, _, _ := x509util.SplitSANs([]string{opts.CommonName, opts.SubjectAltName})
	signed, err := ca.SignCSR(csr, &certutil.CSRRequest{
		ExtKeyUsage: []x509.ExtKeyUsage{usage},
		CommonName:  opts.CommonName,
		DNSNames:    dns,
		IPs:         ips,
		ValidPeriod: opts.ValidPeriod,
	})
	if err != nil {
		return nil, errors.Wrap(err)
	}
	return m.Import(ctx, entity.CertCreateOptions{
		Name:   opts.Name,
		IsCA:   false,
		Type:   entity.CertTypeRemoteCert,
		Bundle: signed,
	})
}

func (m *manager) Get(ctx context.Context, certID int32) (*entity.Certificate, error) {
	cert, err := m.repository.GetCertificate(ctx, certID)
	if err != nil {
		return nil, errors.Wrap(err)
	}
	return describe(cert)
}

// List leaves out the system CAs, which are never listed nor deleted.
func (m *manager) List(ctx context.Context, opts entity.CertListOptions) ([]*entity.Certificate, error) {
	certs, err := m.repository.ListCertificates(ctx, opts)
	if err != nil {
		return nil, errors.Wrap(err)
	}
	result := make([]*entity.Certificate, 0, len(certs))
	for _, cert := range certs {
		if isSystemCA(cert.Name) {
			continue
		}
		described, err := describe(cert)
		if err != nil {
			return nil, errors.Wrapf(err, "Failed to extract cert details for cert %s", cert.Name)
		}
		result = append(result, described)
	}
	return result, nil
}

func (m *manager) Delete(ctx context.Context, certIDs []int32) error {
	err := m.repository.Transaction(ctx, func(_ctx context.Context) error {
		for _, certID := range certIDs {
			cert, err := m.repository.GetCertificate(_ctx, certID)
			if err != nil {
				return errors.Wrap(err)
			}
			if isSystemCA(cert.Name) {
				return errors.BadRequest.Newf("The system CA %s cannot be deleted", cert.Name)
			}
			if cert.RefCount > 0 {
				return errors.Conflict.Newf("The certificate %s cannot be deleted as it is being referenced", cert.Name)
			}
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

// ServerCA signs the manager's own certificates; agents verify the manager against it.
func (m *manager) ServerCA() (certutil.CABundle, error) {
	return m.loaded(&m.serverCA, preset.ServerCAName)
}

// AgentCA signs the certificates agents present; the manager verifies agents against it.
func (m *manager) AgentCA() (certutil.CABundle, error) {
	return m.loaded(&m.agentCA, preset.AgentCAName)
}

func (m *manager) loaded(slot *certutil.CABundle, name string) (certutil.CABundle, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if *slot == nil {
		return nil, errors.Newf("the system CA %s is loaded by Init, which has not run", name)
	}
	return *slot, nil
}

// loadSystemCA is the system CA stored as name, created when there is none. Its
// private key lives in the database only: nothing writes it to disk.
func (m *manager) loadSystemCA(ctx context.Context, name, role string) (certutil.CABundle, error) {
	stored, err := m.repository.GetCertificateByName(ctx, name)
	switch {
	case err == nil:
		ca, ok := stored.Bundle.(certutil.CABundle)
		if !ok {
			return nil, errors.Newf("system CA %s holds no CA bundle", name)
		}
		return ca, nil
	case errors.Is(err, errors.NotFound):
		ca, err := certutil.New(certutil.WithCommonName(strings.TrimSpace(info.ProductName + " " + role)))
		if err != nil {
			return nil, errors.Wrap(err)
		}
		if _, err := m.repository.CreateCertificate(ctx, entity.CertCreateOptions{
			Name:    name,
			IsCA:    true,
			IsLocal: true,
			Type:    entity.CertTypeCertKey,
			Bundle:  ca,
			Source:  entity.CertSourceFactory,
		}); err != nil {
			return nil, errors.Wrapf(err, "Failed to store the system CA %s", name)
		}
		return ca, nil
	default:
		return nil, errors.Wrap(err)
	}
}

func isSystemCA(name string) bool {
	return name == preset.ServerCAName || name == preset.AgentCAName
}
