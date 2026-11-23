package certutil

import (
	"crypto"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// A key parses whichever unencrypted form it comes in: PKCS #8, SEC 1 or PKCS #1.
func TestParseDERKeyReadsEveryUnencryptedForm(t *testing.T) {
	ec, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	require.NoError(t, err)
	rsaKey, err := rsa.GenerateKey(rand.Reader, 2048)
	require.NoError(t, err)
	pkcs8EC, err := x509.MarshalPKCS8PrivateKey(ec)
	require.NoError(t, err)
	sec1, err := x509.MarshalECPrivateKey(ec)
	require.NoError(t, err)

	for name, tc := range map[string]struct {
		der  []byte
		want any
	}{
		"PKCS #8": {pkcs8EC, ec},
		"SEC 1":   {sec1, ec},
		"PKCS #1": {x509.MarshalPKCS1PrivateKey(rsaKey), rsaKey},
	} {
		t.Run(name, func(t *testing.T) {
			key, err := ParseDERKey(tc.der, "")
			require.NoError(t, err)
			assert.True(t, tc.want.(interface {
				Equal(x crypto.PrivateKey) bool
			}).Equal(key))
		})
	}

	_, err = ParseDERKey([]byte("not a key"), "")
	assert.Error(t, err, "neither form fits: PKCS #8's error comes back")
}
