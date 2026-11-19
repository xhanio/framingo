package certificate

import (
	"context"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/x509"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/xhanio/errors"

	"github.com/xhanio/framingo/pkg/utils/certutil"
	"github.com/xhanio/framingo/pkg/utils/testutil"

	"github.com/xhanio/framingo/example/pkg/services/repository"
	"github.com/xhanio/framingo/example/pkg/services/system/platform"
	"github.com/xhanio/framingo/example/pkg/types/entity"
	"github.com/xhanio/framingo/example/pkg/types/orm"
	"github.com/xhanio/framingo/example/pkg/types/preset"
	"github.com/xhanio/framingo/example/pkg/utils/infra"
)

// Init dumps the default CA under infra.ConfigDir, hence the temp dir
func setup(t *testing.T) *manager {
	t.Helper()
	infra.ConfigDir = t.TempDir()
	t.Cleanup(func() { infra.ConfigDir = "" })
	db, err := testutil.SetupDB()
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, db.Cleanup(true)) })
	require.NoError(t, db.ORM().AutoMigrate(&orm.Certificate{}))
	p := platform.New(db, nil, nil, nil)
	m := newManager(p, repository.New(p))
	require.NoError(t, m.Init(context.Background()))
	return m
}

func TestListLeavesOutTheDefaultCA(t *testing.T) {
	m := setup(t)
	ctx := context.Background()

	certs, err := m.List(ctx, entity.CertListOptions{})
	require.NoError(t, err)
	assert.Empty(t, certs, "only the default CA exists yet")

	ca, err := m.repository.GetCertificateByName(ctx, preset.CAName)
	require.NoError(t, err)
	issued, err := m.Issue(ctx, entity.CertIssueOptions{Name: "srv", CommonName: "srv.example.com", CAID: ca.ID})
	require.NoError(t, err)
	assert.Equal(t, "CN=srv.example.com", issued.Subject)

	certs, err = m.List(ctx, entity.CertListOptions{})
	require.NoError(t, err)
	require.Len(t, certs, 1)
	assert.Equal(t, "srv", certs[0].Name)
	assert.NotEmpty(t, certs[0].SerialNumber)
	assert.Equal(t, "MD5", certs[0].Fingerprint.Algorithm)
}

// A CSR is signed into a certificate the manager stores without a key: the
// requester keeps its own.
func TestSignCSRStoresTheCertificateNotTheKey(t *testing.T) {
	m := setup(t)
	ctx := context.Background()
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	require.NoError(t, err)
	csr, err := certutil.NewCSR(key, "agent-7", nil, nil)
	require.NoError(t, err)

	signed, err := m.SignCSR(ctx, entity.CertSignOptions{Name: "agent-7", CSR: csr, CommonName: "agent-7"})
	require.NoError(t, err)
	stored, err := m.Get(ctx, signed.ID)
	require.NoError(t, err)
	assert.Nil(t, stored.Bundle.Key(), "the manager never holds the requester's key")
	assert.False(t, stored.IsLocal)
	assert.True(t, key.PublicKey.Equal(stored.Bundle.Cert().PublicKey))
	assert.Equal(t, []x509.ExtKeyUsage{x509.ExtKeyUsageClientAuth}, stored.Bundle.Cert().ExtKeyUsage, "client by default")
	assert.WithinDuration(t, time.Now().Add(defaultSignValidity), stored.Bundle.Cert().NotAfter, time.Minute)

	ca, err := m.DefaultCA()
	require.NoError(t, err)
	_, err = stored.Bundle.Cert().Verify(x509.VerifyOptions{
		Roots:     certutil.NewCertPool(ca.Cert()),
		KeyUsages: []x509.ExtKeyUsage{x509.ExtKeyUsageClientAuth},
	})
	require.NoError(t, err, "signed by the default CA, which agents are verified against")
}

func TestSignCSRRefusals(t *testing.T) {
	m := setup(t)
	ctx := context.Background()
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	require.NoError(t, err)
	csr, err := certutil.NewCSR(key, "agent-7", nil, nil)
	require.NoError(t, err)
	isA := func(category errors.Category, err error) bool { return err != nil && errors.Is(err, category) }

	_, err = m.SignCSR(ctx, entity.CertSignOptions{Name: "x", CSR: []byte("not a csr"), CommonName: "agent-7"})
	assert.True(t, isA(errors.InvalidArgument, err), "a malformed CSR: got %v", err)
	_, err = m.SignCSR(ctx, entity.CertSignOptions{Name: "x", CSR: csr, CommonName: "agent-8"})
	assert.True(t, isA(errors.InvalidArgument, err), "a CSR for another name: got %v", err)
	_, err = m.SignCSR(ctx, entity.CertSignOptions{Name: "x", CSR: csr, CommonName: "agent-7", Usage: "ca"})
	assert.True(t, isA(errors.InvalidArgument, err), "an unknown usage: got %v", err)
}
