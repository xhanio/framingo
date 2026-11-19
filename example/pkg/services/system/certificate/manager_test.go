package certificate

import (
	"context"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/x509"
	"os"
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
)

func setup(t *testing.T) (*manager, platform.Platform) {
	t.Helper()
	db, err := testutil.SetupDB()
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, db.Cleanup(true)) })
	require.NoError(t, db.ORM().AutoMigrate(&orm.Certificate{}))
	p := platform.New(db, nil, nil, nil)
	m := newManager(p, repository.New(p))
	require.NoError(t, m.Init(context.Background()))
	return m, p
}

func isA(category errors.Category, err error) bool {
	return err != nil && errors.Is(err, category)
}

func verifies(cert *x509.Certificate, ca certutil.CABundle, usage x509.ExtKeyUsage) error {
	_, err := cert.Verify(x509.VerifyOptions{
		Roots:     certutil.NewCertPool(ca.Cert()),
		KeyUsages: []x509.ExtKeyUsage{usage},
	})
	return err
}

// Init creates one CA per purpose and keeps them in the database alone: a
// restarted manager loads the same two, and nothing reaches the disk.
func TestSystemCAs(t *testing.T) {
	dir := t.TempDir()
	t.Chdir(dir)
	m, p := setup(t)

	server, err := m.ServerCA()
	require.NoError(t, err)
	agent, err := m.AgentCA()
	require.NoError(t, err)
	assert.False(t, server.Cert().Equal(agent.Cert()), "independent CAs")
	assert.Contains(t, server.Cert().Subject.CommonName, "server CA")
	assert.Contains(t, agent.Cert().Subject.CommonName, "agent CA")

	restarted := newManager(p, repository.New(p))
	require.NoError(t, restarted.Init(context.Background()))
	reloaded, err := restarted.ServerCA()
	require.NoError(t, err)
	assert.True(t, server.Cert().Equal(reloaded.Cert()), "the server CA is reloaded, not recreated")
	reloaded, err = restarted.AgentCA()
	require.NoError(t, err)
	assert.True(t, agent.Cert().Equal(reloaded.Cert()), "the agent CA is reloaded, not recreated")

	entries, err := os.ReadDir(dir)
	require.NoError(t, err)
	assert.Empty(t, entries, "no CA is written to disk")
}

func TestSystemCAsAreNeitherListedNorDeleted(t *testing.T) {
	m, _ := setup(t)
	ctx := context.Background()

	certs, err := m.List(ctx, entity.CertListOptions{})
	require.NoError(t, err)
	assert.Empty(t, certs, "only the system CAs exist yet")

	for _, name := range []string{preset.ServerCAName, preset.AgentCAName} {
		ca, err := m.repository.GetCertificateByName(ctx, name)
		require.NoError(t, err)
		err = m.Delete(ctx, []int32{ca.ID})
		assert.True(t, isA(errors.BadRequest, err), "deleting %s: got %v", name, err)
	}
}

// Issue signs with the server CA unless told which CA to sign with.
func TestIssue(t *testing.T) {
	m, _ := setup(t)
	ctx := context.Background()
	server, err := m.ServerCA()
	require.NoError(t, err)
	agent, err := m.AgentCA()
	require.NoError(t, err)

	issued, err := m.Issue(ctx, entity.CertIssueOptions{Name: "srv", CommonName: "srv.example.com"})
	require.NoError(t, err)
	assert.Equal(t, "CN=srv.example.com", issued.Subject)
	stored, err := m.Get(ctx, issued.ID)
	require.NoError(t, err)
	require.NoError(t, verifies(stored.Bundle.Cert(), server, x509.ExtKeyUsageServerAuth), "the server CA by default")

	agentCA, err := m.repository.GetCertificateByName(ctx, preset.AgentCAName)
	require.NoError(t, err)
	named, err := m.Issue(ctx, entity.CertIssueOptions{Name: "named", CommonName: "named.example.com", CAID: agentCA.ID})
	require.NoError(t, err)
	stored, err = m.Get(ctx, named.ID)
	require.NoError(t, err)
	require.NoError(t, verifies(stored.Bundle.Cert(), agent, x509.ExtKeyUsageServerAuth), "the CA named")

	_, err = m.Issue(ctx, entity.CertIssueOptions{Name: "x", CommonName: "x", CAID: issued.ID})
	assert.True(t, isA(errors.BadRequest, err), "a CA that is not one: got %v", err)

	certs, err := m.List(ctx, entity.CertListOptions{})
	require.NoError(t, err)
	require.Len(t, certs, 2)
	assert.Equal(t, "srv", certs[0].Name)
	assert.NotEmpty(t, certs[0].SerialNumber)
	assert.Equal(t, "MD5", certs[0].Fingerprint.Algorithm)
}

// A CSR is signed by the agent CA into a certificate the manager stores without
// a key: the requester keeps its own.
func TestSignCSRStoresTheCertificateNotTheKey(t *testing.T) {
	m, _ := setup(t)
	ctx := context.Background()
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	require.NoError(t, err)
	csr, err := certutil.NewCSR(key, "agent-7", nil, nil)
	require.NoError(t, err)

	signed, err := m.SignCSR(ctx, entity.CertSignOptions{Name: "agent-7", CSR: csr, CommonName: "agent-7"})
	require.NoError(t, err)
	stored, err := m.Get(ctx, signed.ID)
	require.NoError(t, err)
	cert := stored.Bundle.Cert()
	assert.Nil(t, stored.Bundle.Key(), "the manager never holds the requester's key")
	assert.False(t, stored.IsLocal)
	assert.True(t, key.PublicKey.Equal(cert.PublicKey))
	assert.Equal(t, []x509.ExtKeyUsage{x509.ExtKeyUsageClientAuth}, cert.ExtKeyUsage, "client by default")
	assert.WithinDuration(t, time.Now().Add(defaultSignValidity), cert.NotAfter, time.Minute)

	agent, err := m.AgentCA()
	require.NoError(t, err)
	require.NoError(t, verifies(cert, agent, x509.ExtKeyUsageClientAuth), "signed by the agent CA")
	server, err := m.ServerCA()
	require.NoError(t, err)
	assert.Error(t, verifies(cert, server, x509.ExtKeyUsageClientAuth), "an agent certificate is no use against the server CA")
}

func TestSignCSRRefusals(t *testing.T) {
	m, _ := setup(t)
	ctx := context.Background()
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	require.NoError(t, err)
	csr, err := certutil.NewCSR(key, "agent-7", nil, nil)
	require.NoError(t, err)

	_, err = m.SignCSR(ctx, entity.CertSignOptions{Name: "x", CSR: []byte("not a csr"), CommonName: "agent-7"})
	assert.True(t, isA(errors.InvalidArgument, err), "a malformed CSR: got %v", err)
	_, err = m.SignCSR(ctx, entity.CertSignOptions{Name: "x", CSR: csr, CommonName: "agent-8"})
	assert.True(t, isA(errors.InvalidArgument, err), "a CSR for another name: got %v", err)
	_, err = m.SignCSR(ctx, entity.CertSignOptions{Name: "x", CSR: csr, CommonName: "agent-7", Usage: "ca"})
	assert.True(t, isA(errors.InvalidArgument, err), "an unknown usage: got %v", err)
}
