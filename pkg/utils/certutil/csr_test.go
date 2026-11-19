package certutil

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/tls"
	"crypto/x509"
	"encoding/pem"
	"net"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/xhanio/errors"
)

func newTestCA(t *testing.T) Manager {
	t.Helper()
	ca, err := New()
	require.NoError(t, err)
	return ca
}

func newAgentKey(t *testing.T) *ecdsa.PrivateKey {
	t.Helper()
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	require.NoError(t, err)
	return key
}

func csrFor(t *testing.T, key *ecdsa.PrivateKey, cn string, dnsNames []string, ips []net.IP) *x509.CertificateRequest {
	t.Helper()
	data, err := NewCSR(key, cn, dnsNames, ips)
	require.NoError(t, err)
	csr, err := ParseCSR(data)
	require.NoError(t, err)
	return csr
}

var client = []x509.ExtKeyUsage{x509.ExtKeyUsageClientAuth}

func isInvalidArgument(err error) bool {
	return err != nil && errors.Is(err, errors.InvalidArgument)
}

// The requester keeps its key: the CA signs the CSR's public key and hands back
// a certificate only, which the requester pairs with its own key.
func TestSignCSRCertifiesTheRequestersKey(t *testing.T) {
	ca := newTestCA(t)
	key := newAgentKey(t)

	signed, err := ca.SignCSR(csrFor(t, key, "agent-7", nil, nil), &CSRRequest{
		ExtKeyUsage: client,
		CommonName:  "agent-7",
		ValidPeriod: 24 * time.Hour,
	})
	require.NoError(t, err)
	assert.Nil(t, signed.Key(), "the CA never holds the requester's key")
	assert.Empty(t, signed.KeyPEM())

	cert := signed.Cert()
	assert.True(t, key.PublicKey.Equal(cert.PublicKey), "the certificate is for the requester's key")
	assert.Equal(t, "agent-7", cert.Subject.CommonName)
	assert.Equal(t, []x509.ExtKeyUsage{x509.ExtKeyUsageClientAuth}, cert.ExtKeyUsage)
	assert.Zero(t, cert.KeyUsage&x509.KeyUsageKeyEncipherment, "no RSA key exchange for an ECDSA key")
	assert.WithinDuration(t, time.Now().Add(24*time.Hour), cert.NotAfter, time.Minute)

	_, err = cert.Verify(x509.VerifyOptions{
		Roots:     NewCertPool(ca.Cert()),
		KeyUsages: []x509.ExtKeyUsage{x509.ExtKeyUsageClientAuth},
	})
	require.NoError(t, err, "the certificate chains to the CA as a client certificate")

	keyDER, err := x509.MarshalPKCS8PrivateKey(key)
	require.NoError(t, err)
	_, err = tls.X509KeyPair(signed.CertPEM(), pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: keyDER}))
	require.NoError(t, err, "the requester pairs the certificate with its own key")
}

// The CA decides who a certificate is for: a CSR may ask only for names the
// request grants.
func TestSignCSRRefusesNamesNotGranted(t *testing.T) {
	ca := newTestCA(t)
	key := newAgentKey(t)
	grant := &CSRRequest{ExtKeyUsage: client, CommonName: "agent-7", DNSNames: []string{"agent-7.local"}, IPs: []net.IP{net.ParseIP("10.0.0.7")}}

	for name, csr := range map[string]*x509.CertificateRequest{
		"another common name": csrFor(t, key, "agent-8", nil, nil),
		"a DNS name":          csrFor(t, key, "agent-7", []string{"manager.local"}, nil),
		"an IP":               csrFor(t, key, "agent-7", nil, []net.IP{net.ParseIP("10.0.0.8")}),
	} {
		_, err := ca.SignCSR(csr, grant)
		assert.True(t, isInvalidArgument(err), "%s: got %v", name, err)
	}

	signed, err := ca.SignCSR(csrFor(t, key, "", []string{"agent-7.local"}, []net.IP{net.ParseIP("10.0.0.7")}), grant)
	require.NoError(t, err, "a CSR naming nothing, or only granted names, is signed")
	assert.Equal(t, "agent-7", signed.Cert().Subject.CommonName, "the request supplies the common name")

	_, err = ca.SignCSR(csrFor(t, key, "agent-7", nil, nil), &CSRRequest{ExtKeyUsage: client})
	assert.True(t, isInvalidArgument(err), "no common name granted: got %v", err)
}

func TestSignCSRExtKeyUsage(t *testing.T) {
	ca := newTestCA(t)
	key := newAgentKey(t)

	signed, err := ca.SignCSR(csrFor(t, key, "manager.local", nil, nil), &CSRRequest{ExtKeyUsage: []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth}, CommonName: "manager.local"})
	require.NoError(t, err)
	assert.Equal(t, []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth}, signed.Cert().ExtKeyUsage)
	assert.Equal(t, ca.Cert().NotAfter, signed.Cert().NotAfter, "no valid period lasts as long as the CA")

	_, err = ca.SignCSR(csrFor(t, key, "x", nil, nil), &CSRRequest{CommonName: "x"})
	assert.True(t, isInvalidArgument(err), "no extended key usage: got %v", err)
}

// A CSR's signature proves its sender holds the private key; one that does not
// verify is refused.
func TestParseCSRRefusesATamperedRequest(t *testing.T) {
	data, err := NewCSR(newAgentKey(t), "agent-7", nil, nil)
	require.NoError(t, err)
	block, _ := pem.Decode(data)
	der := append([]byte(nil), block.Bytes...)
	der[len(der)-1] ^= 0xff // the signature's last byte

	_, err = ParseCSR(der)
	assert.True(t, isInvalidArgument(err), "a tampered signature: got %v", err)

	_, err = ParseCSR(newTestCA(t).CertPEM())
	assert.True(t, isInvalidArgument(err), "a certificate, not a request: got %v", err)

	csr, err := ParseCSR(block.Bytes)
	require.NoError(t, err, "DER is accepted as well as PEM")
	assert.Equal(t, "agent-7", csr.Subject.CommonName)
}

func TestSignServerHonoursValidPeriod(t *testing.T) {
	signed, err := newTestCA(t).SignServer(&ServerRequest{CommonName: "manager.local", ValidPeriod: time.Hour})
	require.NoError(t, err)
	assert.WithinDuration(t, time.Now().Add(time.Hour), signed.Cert().NotAfter, time.Minute)
}

func TestSerialNumbersAreRandom(t *testing.T) {
	ca := newTestCA(t)
	key := newAgentKey(t)
	seen := map[string]bool{}
	for range 16 {
		signed, err := ca.SignCSR(csrFor(t, key, "agent-7", nil, nil), &CSRRequest{ExtKeyUsage: client, CommonName: "agent-7"})
		require.NoError(t, err)
		sn := signed.Cert().SerialNumber
		assert.Greater(t, sn.BitLen(), 64, "not a timestamp")
		assert.False(t, seen[sn.String()], "serial %s repeated", sn)
		seen[sn.String()] = true
	}
}
