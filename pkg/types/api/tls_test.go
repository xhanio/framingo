package api

import (
	"crypto/x509"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"strconv"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/xhanio/framingo/pkg/utils/certutil"
)

func newCA(t *testing.T) certutil.Manager {
	t.Helper()
	ca, err := certutil.New()
	require.NoError(t, err)
	return ca
}

func clientCert(t *testing.T, ca certutil.CABundle) certutil.CertBundle {
	t.Helper()
	b, err := ca.SignClient(&certutil.ClientRequest{CommonName: "agent-7"})
	require.NoError(t, err)
	return b
}

// Each side verifies the other against the CAs it is given, and nothing else:
// the server agents by the agent CA, the agent the server by the server CA.
func TestTLSVerifiesBothWays(t *testing.T) {
	serverCA, agentCA := newCA(t), newCA(t)
	serverCert, err := serverCA.SignServer(&certutil.ServerRequest{CommonName: "127.0.0.1", IPs: []net.IP{net.ParseIP("127.0.0.1")}})
	require.NoError(t, err)

	srv := httptest.NewUnstartedServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		io.WriteString(w, strconv.Itoa(len(r.TLS.VerifiedChains))) //nolint:errcheck // the client's read fails instead
	}))
	srv.TLS = (&ServerTLS{CertBundle: serverCert, ClientCAs: []*x509.Certificate{agentCA.Cert()}}).AsConfig()
	srv.StartTLS()
	t.Cleanup(srv.Close)

	get := func(ct *ClientTLS) (string, error) {
		cli := &http.Client{Transport: &http.Transport{TLSClientConfig: ct.AsConfig()}}
		resp, err := cli.Get(srv.URL)
		if err != nil {
			return "", err
		}
		defer resp.Body.Close()
		body, err := io.ReadAll(resp.Body)
		return string(body), err
	}
	trustServer := []*x509.Certificate{serverCA.Cert()}

	chains, err := get(&ClientTLS{CertBundle: clientCert(t, agentCA), RootCAs: trustServer})
	require.NoError(t, err)
	assert.Equal(t, "1", chains, "an agent certificate is verified")

	chains, err = get(&ClientTLS{RootCAs: trustServer})
	require.NoError(t, err)
	assert.Equal(t, "0", chains, "a client certificate is asked for, not required")

	_, err = get(&ClientTLS{CertBundle: clientCert(t, serverCA), RootCAs: trustServer})
	assert.ErrorContains(t, err, "remote error: tls: unknown certificate authority", "the server refuses a client certificate from another CA")

	_, err = get(&ClientTLS{CertBundle: clientCert(t, agentCA), RootCAs: []*x509.Certificate{agentCA.Cert()}})
	assert.ErrorContains(t, err, "x509: certificate signed by unknown authority", "the client's own CA does not vouch for the server")

	_, err = get(nil)
	assert.ErrorContains(t, err, "x509: certificate signed by unknown authority", "no CAs given is the system's, not no verification")
}

func TestServerTLSAsksForNoClientCertWithoutCAs(t *testing.T) {
	cfg := (&ServerTLS{}).AsConfig()
	assert.Zero(t, cfg.ClientAuth)
	assert.Nil(t, cfg.ClientCAs)
}
