package certificate

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/xhanio/framingo/pkg/utils/testutil"

	"github.com/xhanio/framingo/example/pkg/services/repository"
	"github.com/xhanio/framingo/example/pkg/services/system/platform"
	"github.com/xhanio/framingo/example/pkg/types/entity"
	"github.com/xhanio/framingo/example/pkg/types/orm"
	"github.com/xhanio/framingo/example/pkg/types/preset"
	"github.com/xhanio/framingo/example/pkg/utils/infra"
)

// setup is the service over a real test database, initialised as the
// supervisor would - which creates the default CA. Init dumps the CA under
// infra.ConfigDir, so that points at a temp dir rather than the source tree.
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

// The default CA is the system's own: it signs what Issue issues, but List
// leaves it out. That is the service's rule now, not a condition of the
// repository's query - and what List shows is described from the bundle the
// repository decoded.
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
