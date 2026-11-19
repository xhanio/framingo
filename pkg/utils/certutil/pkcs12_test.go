package certutil

import (
	"crypto"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestEncodePKCS12RoundTrips(t *testing.T) {
	ca := newTestCA(t)
	signed, err := ca.SignServer(&ServerRequest{CommonName: "srv.local", KeepChain: true})
	require.NoError(t, err)

	data, err := EncodePKCS12(signed, "s3cret")
	require.NoError(t, err)
	cert, chain, key, err := ParsePKCS12(data, "s3cret")
	require.NoError(t, err)
	assert.True(t, signed.Cert().Equal(cert))
	assert.True(t, key.(interface{ Equal(crypto.PrivateKey) bool }).Equal(signed.Key()))
	require.Len(t, chain, 1)
	assert.True(t, ca.Cert().Equal(chain[0]), "the chain travels with it")

	_, _, _, err = ParsePKCS12(data, "guess")
	assert.Error(t, err, "the password encrypts it")
}

func TestEncodePKCS12NeedsAKey(t *testing.T) {
	signed, err := newTestCA(t).SignServer(&ServerRequest{CommonName: "srv.local"})
	require.NoError(t, err)
	certOnly, err := NewCertBundleWithoutKey(signed.CertPEM())
	require.NoError(t, err)

	_, err = EncodePKCS12(certOnly, "s3cret")
	assert.True(t, isInvalidArgument(err), "a certificate without its key: got %v", err)
}
