package certutil

import (
	"crypto"
	"crypto/tls"
	"crypto/x509"
	"encoding/pem"
	"fmt"
	"io"
	"os"

	"github.com/xhanio/errors"
)

type bundle struct {
	certPEM []byte
	keyPEM  []byte
	keyDER  []byte

	cert *x509.Certificate
	pool []*x509.Certificate
	key  crypto.PrivateKey

	tc tls.Certificate
}

func newCABundle(cn string) (*bundle, error) {
	key, err := generateKey()
	if err != nil {
		return nil, errors.Wrap(err)
	}
	cert, err := generateCA(cn, key)
	if err != nil {
		return nil, errors.Wrap(err)
	}
	result := &bundle{
		cert: cert,
		key:  key,
	}
	return result, nil
}

func newBundleFromFile(certFile, caFile, keyFile string) (*bundle, error) {
	var certBytes, caBytes, keyBytes []byte
	var err error
	if certFile != "" {
		certBytes, err = os.ReadFile(certFile)
		if err != nil {
			return nil, errors.Wrapf(err, "failed to read cert file %s", certFile)
		}
	}
	if caFile != "" {
		caBytes, err = os.ReadFile(caFile)
		if err != nil {
			return nil, errors.Wrapf(err, "failed to read ca file %s", certFile)
		}
	}
	if keyFile != "" {
		keyBytes, err = os.ReadFile(keyFile)
		if err != nil {
			return nil, errors.Wrapf(err, "failed to read key file %s", keyFile)
		}
	}
	b, err := newBundleFromBytes(certBytes, caBytes, keyBytes, "")
	if err != nil {
		return nil, errors.Wrap(err)
	}
	if keyFile != "" && b.key == nil {
		return nil, errors.NotFound.Newf("private key not found")
	}
	return b, nil
}

func newBundleFromBytes(certBytes, caBytes, keyBytes []byte, password string) (*bundle, error) {
	result := &bundle{}
	if len(certBytes) == 0 && len(caBytes) == 0 && len(keyBytes) == 0 {
		return nil, errors.Newf("empty cert bytes")
	} else if len(caBytes) == 0 && len(keyBytes) == 0 {
		// try pkcs12 first
		if cert, pool, key, err := ParsePKCS12(certBytes, password); err == nil {
			result.cert = cert
			result.pool = pool
			result.key = key
		}
		if result.cert != nil {
			if err := result.init(); err != nil {
				return nil, errors.Wrap(err)
			}
			return result, nil
		}
		// then pem
		if cert, pool, key, err := ParsePEM(certBytes, nil, nil, password); err == nil {
			result.cert = cert
			result.pool = pool
			result.key = key
		}
		if result.cert != nil {
			if err := result.init(); err != nil {
				return nil, errors.Wrap(err)
			}
			return result, nil
		}
		// then der
		if cert, err := ParseDERCert(certBytes); err == nil {
			result.cert = cert
		}
		if result.cert != nil {
			if err := result.init(); err != nil {
				return nil, errors.Wrap(err)
			}
			return result, nil
		}
	} else {
		// try pem first
		if cert, pool, key, err := ParsePEM(certBytes, caBytes, keyBytes, password); err == nil {
			result.cert = cert
			result.pool = pool
			result.key = key
		}
		if result.cert != nil {
			if err := result.init(); err != nil {
				return nil, errors.Wrap(err)
			}
			return result, nil
		}
		// then der
		if cert, err := ParseDERCert(certBytes); err == nil {
			result.cert = cert
		}
		if key, err := ParseDERKey(keyBytes, password); err == nil {
			result.key = key
		}
		if result.cert != nil {
			if err := result.init(); err != nil {
				return nil, errors.Wrap(err)
			}
			return result, nil
		}
	}
	return nil, errors.NotFound.Newf("failed to parse cert bytes: no valid cert found")
}

func (b *bundle) init() error {
	if b.cert != nil {
		b.certPEM = append(b.certPEM, pem.EncodeToMemory(&pem.Block{
			Type:  "CERTIFICATE",
			Bytes: b.cert.Raw,
		})...)
		for _, ca := range b.pool {
			b.certPEM = append(b.certPEM, pem.EncodeToMemory(&pem.Block{
				Type:  "CERTIFICATE",
				Bytes: ca.Raw,
			})...)
		}
	}
	if b.key != nil {
		keyDER, err := x509.MarshalPKCS8PrivateKey(b.key)
		if err != nil {
			return errors.Wrap(err)
		}
		b.keyDER = keyDER
		b.keyPEM = pem.EncodeToMemory(&pem.Block{
			Type:  "PRIVATE KEY",
			Bytes: keyDER,
		})
	}
	return nil
}

// parse tls cert
func (b *bundle) initTLS() error {
	if len(b.certPEM) == 0 || len(b.keyPEM) == 0 {
		err := b.init()
		if err != nil {
			return errors.Newf("invalid tls certificate")
		}
	}
	tlsCert, err := tls.X509KeyPair(b.certPEM, b.keyPEM)
	if err != nil {
		return errors.Wrap(err)
	}
	b.tc = tlsCert
	return nil
}

func (b *bundle) CAs() []*x509.Certificate {
	return b.pool
}

func (b *bundle) IsCA() bool {
	if b.cert == nil {
		return false
	}
	return b.cert.IsCA
}

func (b *bundle) Cert() *x509.Certificate {
	return b.cert
}

func (b *bundle) CertDER() []byte {
	if b.cert == nil {
		return nil
	}
	return b.cert.Raw
}

func (b *bundle) CertPEM() []byte {
	return b.certPEM
}

func (b *bundle) CertTLS() tls.Certificate {
	return b.tc
}

func (b *bundle) Key() crypto.PrivateKey {
	return b.key
}

func (b *bundle) KeyDER() []byte {
	return b.keyDER
}

func (b *bundle) KeyPEM() []byte {
	return b.keyPEM
}

func (b *bundle) SignCA(req *CARequest) (CABundle, error) {
	if b.cert == nil {
		return nil, errors.Newf("unable to sign ca from bundle: cert is empty")
	}
	if !b.cert.IsCA {
		return nil, errors.Newf("unable to sign ca from bundle: bundle is not a ca")
	}
	key, err := generateKey()
	if err != nil {
		return nil, errors.Wrap(err)
	}
	cert, err := signCA(req, key, b)
	if err != nil {
		return nil, errors.Wrap(err)
	}
	result := &bundle{
		cert: cert,
		key:  key,
	}
	if req.KeepChain {
		result.pool = append(result.pool, b.cert)
		result.pool = append(result.pool, b.pool...)
	}
	err = result.initTLS()
	if err != nil {
		return nil, errors.Wrap(err)
	}
	return result, nil
}

func (b *bundle) SignServer(req *ServerRequest) (CertBundle, error) {
	if !b.cert.IsCA {
		return nil, errors.Newf("unable to sign cert from bundle: bundle is not a ca")
	}
	key, err := generateKey()
	if err != nil {
		return nil, errors.Wrap(err)
	}
	cert, err := signServer(req, key, b)
	if err != nil {
		return nil, errors.Wrap(err)
	}
	result := &bundle{
		cert: cert,
		key:  key,
	}
	if req.KeepChain {
		result.pool = append(result.pool, b.cert)
		result.pool = append(result.pool, b.pool...)
	}
	err = result.initTLS()
	if err != nil {
		return nil, errors.Wrap(err)
	}
	return result, nil
}

func (b *bundle) SignClient(req *ClientRequest) (CertBundle, error) {
	if !b.cert.IsCA {
		return nil, errors.Newf("unable to sign cert from bundle: bundle is not a ca")
	}
	key, err := generateKey()
	if err != nil {
		return nil, errors.Wrap(err)
	}
	cert, err := signClient(req, key, b)
	if err != nil {
		return nil, errors.Wrap(err)
	}
	result := &bundle{
		key:  key,
		cert: cert,
	}
	if req.KeepChain {
		result.pool = append(result.pool, b.cert)
		result.pool = append(result.pool, b.pool...)
	}
	err = result.initTLS()
	if err != nil {
		return nil, errors.Wrap(err)
	}
	return result, nil
}

func (b *bundle) Dump(certFile, keyFile string) error {
	cf, err := os.Create(certFile)
	if err != nil {
		return errors.Wrap(err)
	}
	if _, err := cf.Write(b.CertPEM()); err != nil {
		ce := cf.Close()
		return errors.Combine(ce, err)
	}
	kf, err := os.Create(keyFile)
	if err != nil {
		return errors.Wrap(err)
	}
	if _, err := kf.Write(b.KeyPEM()); err != nil {
		ke := kf.Close()
		return errors.Combine(ke, err)
	}
	return nil
}

func (b *bundle) Info(w io.Writer, debug bool) {
	fmt.Printf("SN: %s\n", b.cert.SerialNumber)
	fmt.Printf("len certPEM: %d\n", len(b.CertPEM()))
	fmt.Printf("len keyPEM: %d\n", len(b.KeyPEM()))
	fmt.Printf("Root SN: %s\n", b.cert.SerialNumber)
}
