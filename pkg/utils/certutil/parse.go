package certutil

import (
	"crypto"
	"crypto/x509"
	"encoding/pem"
	"strings"

	"github.com/youmark/pkcs8"
	"software.sslmate.com/src/go-pkcs12"

	"github.com/xhanio/errors"
)

func ParsePEMCert(b []byte) (*x509.Certificate, []*x509.Certificate, error) {
	var certs []*x509.Certificate
	curr := b
	var block *pem.Block
	var rest []byte
	for len(curr) > 0 {
		block, rest = pem.Decode(curr)
		if block == nil {
			break
		}
		if strings.Contains(block.Type, "CERTIFICATE") {
			cert, err := ParseDERCert(block.Bytes)
			if err != nil {
				return nil, nil, errors.Wrap(err)
			}
			certs = append(certs, cert)
		}
		curr = rest
	}
	if len(certs) == 1 {
		return certs[0], nil, nil
	} else if len(certs) > 1 {
		return certs[0], certs[1:], nil
	}
	return nil, nil, nil
}

func ParseDERCert(b []byte) (*x509.Certificate, error) {
	cert, err := x509.ParseCertificate(b)
	if err != nil {
		return nil, errors.Wrap(err)
	}
	return cert, nil
}

func ParsePEMKey(b []byte, password string) (crypto.PrivateKey, error) {
	curr := b
	var block *pem.Block
	var rest []byte
	for len(curr) > 0 {
		block, rest = pem.Decode(curr)
		if block == nil {
			break
		}
		if strings.Contains(block.Type, "PRIVATE KEY") {
			pwd := password
			if x509.IsEncryptedPEMBlock(block) {
				b, err := x509.DecryptPEMBlock(block, []byte(password))
				if err != nil {
					return nil, errors.Wrap(err)
				}
				block.Bytes = b
				pwd = ""
			}
			key, err := ParseDERKey(block.Bytes, pwd)
			if err != nil {
				return nil, errors.Wrap(err)
			}
			return key, nil
		}
		curr = rest
	}
	return nil, nil
}

func ParseDERKey(der []byte, password string) (crypto.PrivateKey, error) {
	var _key any
	var err error
	_key, err = pkcs8.ParsePKCS8PrivateKey(der, []byte(password))
	if err != nil {
		if strings.Contains(err.Error(), "ParseECPrivateKey") {
			_key, err = x509.ParseECPrivateKey(der)
		} else if strings.Contains(err.Error(), "ParsePKCS1PrivateKey") {
			_key, err = x509.ParsePKCS1PrivateKey(der)
		}
	}
	if err != nil {
		return nil, errors.Wrap(err)
	}
	key, ok := _key.(crypto.PrivateKey)
	if !ok {
		return nil, errors.Newf("key is not a valid crypto.PrivateKey")
	}
	return key, nil
}

func ParsePEM(certBytes, caBytes, keyBytes []byte, password string) (*x509.Certificate, []*x509.Certificate, crypto.PrivateKey, error) {
	var cert *x509.Certificate
	var pool []*x509.Certificate
	var key crypto.PrivateKey
	if len(certBytes) > 0 {
		c, p, err := ParsePEMCert(certBytes)
		if err != nil {
			return nil, nil, nil, errors.Wrap(err)
		} else if c != nil {
			cert = c
			pool = p
		}
		k, err := ParsePEMKey(certBytes, password)
		if err != nil {
			return nil, nil, nil, errors.Wrap(err)
		} else if k != nil {
			key = k
		}
	}
	if len(keyBytes) > 0 {
		k, err := ParsePEMKey(keyBytes, password)
		if err != nil {
			return nil, nil, nil, errors.Wrap(err)
		} else if k != nil {
			key = k
		}
	}
	if len(caBytes) > 0 {
		if c, p, err := ParsePEMCert(caBytes); err != nil {
			return nil, nil, nil, errors.Wrap(err)
		} else if c != nil {
			pool = append(pool, c)
			pool = append(pool, p...)
		}
	}
	return cert, pool, key, nil
}

func ParsePKCS12(pfxBytes []byte, password string) (*x509.Certificate, []*x509.Certificate, crypto.PrivateKey, error) {
	_key, cert, pool, err := pkcs12.DecodeChain(pfxBytes, password)
	if err != nil {
		return nil, nil, nil, errors.Wrap(err)
	}
	key, ok := _key.(crypto.PrivateKey)
	if !ok {
		return nil, nil, nil, errors.Newf("key is not a valid crypto.PrivateKey")
	}
	return cert, pool, key, nil
}
