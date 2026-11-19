package certutil

import (
	"crypto"
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/json"
	"encoding/pem"
	"math/big"
	"net"
	"time"

	"github.com/xhanio/errors"
	"software.sslmate.com/src/go-pkcs12"

	"github.com/xhanio/framingo/pkg/utils/timeutil"
)

func generateKey() (*rsa.PrivateKey, error) {
	key, err := rsa.GenerateKey(rand.Reader, Bits2048)
	if err != nil {
		return nil, errors.Wrap(err)
	}
	return key, nil
}

func generateCA(cn string, key *rsa.PrivateKey) (*x509.Certificate, error) {
	sn, err := newSerial()
	if err != nil {
		return nil, errors.Wrap(err)
	}
	template := &x509.Certificate{
		SerialNumber: sn,
		Subject: pkix.Name{
			CommonName: cn,
		},
		NotBefore:             time.Now().Add(-10 * time.Second),
		NotAfter:              time.Now().AddDate(10, 0, 0),
		BasicConstraintsValid: true,
		IsCA:                  true,
	}
	certDER, err := x509.CreateCertificate(rand.Reader, template, template, &key.PublicKey, key)
	if err != nil {
		return nil, errors.Wrap(err)
	}
	cert, err := x509.ParseCertificate(certDER)
	if err != nil {
		return nil, errors.Wrap(err)
	}
	return cert, nil
}

func signCA(req *CARequest, key *rsa.PrivateKey, ca *bundle) (*x509.Certificate, error) {
	subject := ca.cert.Subject
	subject.CommonName = req.CommonName // overwrite common name

	cr := &x509.CertificateRequest{
		PublicKey:          key.PublicKey,
		SignatureAlgorithm: x509.SHA256WithRSA,
		Subject:            subject,
		DNSNames:           req.DNSNames,
	}
	if len(req.IPs) > 0 {
		cr.IPAddresses = req.IPs
	}
	crb, err := x509.CreateCertificateRequest(rand.Reader, cr, key)
	if err != nil {
		return nil, errors.Wrapf(err, "failed to create certificate request")
	}
	pcr, err := x509.ParseCertificateRequest(crb)
	if err != nil {
		return nil, errors.Wrapf(err, "failed to parse certificate request")
	}

	sn, err := newSerial()
	if err != nil {
		return nil, errors.Wrap(err)
	}

	template := &x509.Certificate{
		IsCA:                  true,
		BasicConstraintsValid: true,
		Signature:             pcr.Signature,
		SignatureAlgorithm:    pcr.SignatureAlgorithm,
		PublicKeyAlgorithm:    pcr.PublicKeyAlgorithm,
		PublicKey:             pcr.PublicKey,
		SerialNumber:          sn,
		Subject:               pcr.Subject,
		NotBefore:             ca.cert.NotBefore,
		NotAfter:              ca.cert.NotAfter,
		KeyUsage:              x509.KeyUsageDigitalSignature | x509.KeyUsageKeyEncipherment | x509.KeyUsageCertSign | x509.KeyUsageCRLSign,
		DNSNames:              pcr.DNSNames,
		IPAddresses:           pcr.IPAddresses,
	}
	certDER, err := x509.CreateCertificate(rand.Reader, template, ca.cert, pcr.PublicKey, ca.Key())
	if err != nil {
		return nil, errors.Wrapf(err, "failed to create certificate")
	}
	cert, err := x509.ParseCertificate(certDER)
	if err != nil {
		return nil, errors.Wrapf(err, "failed to parse certificate")
	}
	return cert, nil
}

// newSerial is random, within RFC 5280's 20 octets.
func newSerial() (*big.Int, error) {
	sn, err := rand.Int(rand.Reader, big.NewInt(1).Lsh(big.NewInt(1), 159))
	if err != nil {
		return nil, errors.Wrapf(err, "failed to generate certificate serial number")
	}
	return sn, nil
}

func signServer(req *ServerRequest, key *rsa.PrivateKey, ca *bundle) (*x509.Certificate, error) {
	return signLeaf(ca, &key.PublicKey, req.CommonName, req.DNSNames, req.IPs, []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth}, req.ValidPeriod)
}

func signClient(req *ClientRequest, key *rsa.PrivateKey, ca *bundle) (*x509.Certificate, error) {
	return signLeaf(ca, &key.PublicKey, req.CommonName, nil, nil, []x509.ExtKeyUsage{x509.ExtKeyUsageClientAuth}, req.ValidPeriod)
}

// signLeaf issues ca's certificate for pub, under ca's subject with cn; a zero
// validPeriod lasts as long as ca.
func signLeaf(ca *bundle, pub crypto.PublicKey, cn string, dnsNames []string, ips []net.IP, ekus []x509.ExtKeyUsage, validPeriod time.Duration) (*x509.Certificate, error) {
	if ca.Key() == nil {
		return nil, errors.Newf("failed to sign certificate: no private key found")
	}
	if len(ekus) == 0 {
		return nil, errors.InvalidArgument.Newf("no extended key usage to sign the certificate for")
	}
	keyUsage := x509.KeyUsageDigitalSignature
	if _, ok := pub.(*rsa.PublicKey); ok {
		keyUsage |= x509.KeyUsageKeyEncipherment // RSA key exchange; meaningless for other keys
	}
	sn, err := newSerial()
	if err != nil {
		return nil, errors.Wrap(err)
	}
	subject := ca.cert.Subject
	subject.CommonName = cn
	template := &x509.Certificate{
		SerialNumber: sn,
		Subject:      subject,
		NotBefore:    ca.cert.NotBefore,
		NotAfter:     ca.cert.NotAfter,
		KeyUsage:     keyUsage,
		ExtKeyUsage:  ekus,
		DNSNames:     dnsNames,
		IPAddresses:  ips,
	}
	if validPeriod > 0 {
		template.NotAfter = timeutil.Earliest(true, ca.cert.NotAfter, time.Now().Add(validPeriod))
	}
	certDER, err := x509.CreateCertificate(rand.Reader, template, ca.cert, pub, ca.Key())
	if err != nil {
		return nil, errors.Wrapf(err, "failed to create certificate")
	}
	cert, err := x509.ParseCertificate(certDER)
	if err != nil {
		return nil, errors.Wrapf(err, "failed to parse certificate")
	}
	return cert, nil
}

// NewCSR is a PEM certificate request for key's public key - what a requester
// sends to have it signed while keeping key to itself.
func NewCSR(key crypto.Signer, cn string, dnsNames []string, ips []net.IP) ([]byte, error) {
	der, err := x509.CreateCertificateRequest(rand.Reader, &x509.CertificateRequest{
		Subject:     pkix.Name{CommonName: cn},
		DNSNames:    dnsNames,
		IPAddresses: ips,
	}, key)
	if err != nil {
		return nil, errors.Wrapf(err, "failed to create certificate request")
	}
	return pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE REQUEST", Bytes: der}), nil
}

func Encode(b CertBundle) ([]byte, error) {
	if b == nil {
		return nil, errors.Newf("The bundle is empty, nothing to encode")
	}

	encodedBundle := map[string]string{}

	if b.Cert() != nil {
		encodedBundle["certPEM"] = string(b.CertPEM())
	}

	if b.Key() != nil {
		encodedBundle["keyPEM"] = string(b.KeyPEM())
	}

	return json.Marshal(encodedBundle)
}

func Decode(bundleData []byte) (CertBundle, error) {
	var decodedBundle map[string]string
	if err := json.Unmarshal(bundleData, &decodedBundle); err != nil {
		return nil, err
	}

	b := &bundle{}
	if certPEM, ok := decodedBundle["certPEM"]; ok {
		b.certPEM = []byte(certPEM)
	}
	if keyPEM, ok := decodedBundle["keyPEM"]; ok {
		b.keyPEM = []byte(keyPEM)
	}

	if len(b.certPEM) > 0 {
		cert, pool, err := ParsePEMCert(b.certPEM)
		if err != nil {
			// Try to parse as DER format
			if cert, err = ParseDERCert(b.certPEM); err != nil {
				return nil, errors.Wrapf(err, "failed to parse certificate as PEM or DER format")
			}
		}
		b.cert = cert
		b.pool = pool
	}
	if len(b.keyPEM) > 0 {
		key, err := ParsePEMKey(b.keyPEM, "")
		if err != nil {
			// Try to parse as DER format
			if key, err = ParseDERKey(b.keyPEM, ""); err != nil {
				return nil, errors.Wrapf(err, "failed to parse key as PEM or DER format")
			}
		}
		b.key = key
	}

	return b, nil
}

// EncodePKCS12 is b's certificate, key and chain as a PKCS#12 file encrypted
// with password - by modern algorithms, AES-256 and PBKDF2-SHA256, which
// OpenSSL 3 reads without -legacy.
func EncodePKCS12(b CertBundle, password string) ([]byte, error) {
	if b.Key() == nil {
		return nil, errors.InvalidArgument.Newf("no private key to encode into PKCS#12")
	}
	data, err := pkcs12.Modern2023.Encode(b.Key(), b.Cert(), b.CAs(), password)
	if err != nil {
		return nil, errors.Wrapf(err, "failed to encode PKCS#12")
	}
	return data, nil
}

func NewCABundle(certBytes []byte, keyPEM []byte) (CABundle, error) {
	b, err := newBundleFromBytes(certBytes, nil, keyPEM, "")
	if err != nil {
		return nil, errors.Wrap(err)
	}
	if !b.cert.IsCA {
		return nil, errors.Newf("failed to create ca bundle: bundle is not ca")
	}
	err = b.initTLS()
	if err != nil {
		return nil, errors.Wrap(err)
	}
	return b, nil
}

func NewCABundleWithoutKey(caBytes []byte) (CertBundle, error) {
	b, err := newBundleFromBytes(caBytes, nil, nil, "")
	if err != nil {
		return nil, errors.Wrap(err)
	}
	if !b.cert.IsCA {
		return nil, errors.Newf("failed to create ca bundle: bundle is not ca")
	}
	return b, nil
}

func NewCertBundle(certBytes, keyBytes []byte) (CertBundle, error) {
	b, err := newBundleFromBytes(certBytes, nil, keyBytes, "")
	if err != nil {
		return nil, errors.Wrap(err)
	}
	if b.cert.IsCA {
		return nil, errors.Newf("failed to create cert bundle: bundle is ca")
	}
	err = b.initTLS()
	if err != nil {
		return nil, errors.Wrap(err)
	}
	return b, nil
}

func NewCertBundleWithoutKey(certBytes []byte) (CertBundle, error) {
	b, err := newBundleFromBytes(certBytes, nil, nil, "")
	if err != nil {
		return nil, errors.Wrap(err)
	}
	if b.cert.IsCA {
		return nil, errors.Newf("failed to create cert bundle: bundle is ca")
	}
	return b, nil
}

func NewCertPool(certs ...*x509.Certificate) *x509.CertPool {
	pool := x509.NewCertPool()
	for _, cert := range certs {
		pool.AddCert(cert)
	}
	return pool
}

func MustCAFromFile(certFile, caFile, keyFile string) CABundle {
	b, err := newBundleFromFile(certFile, caFile, keyFile)
	if err != nil {
		panic(err)
	}
	if !b.cert.IsCA {
		panic(err)
	}
	return b
}

func MustCertFromFile(certFile, caFile, keyFile string) CertBundle {
	b, err := newBundleFromFile(certFile, caFile, keyFile)
	if err != nil {
		panic(err)
	}
	return b
}

func NewCert(cert *x509.Certificate, pool []*x509.Certificate, key crypto.PrivateKey) (CertBundle, error) {
	if cert == nil {
		return nil, errors.Newf("cert must not be empty")
	}
	b := &bundle{
		cert: cert,
		pool: pool,
		key:  key,
	}
	err := b.init()
	if err != nil {
		return nil, errors.Wrap(err)
	}
	if key != nil {
		err := b.initTLS()
		if err != nil {
			return nil, errors.Wrap(err)
		}
	}
	return b, nil
}

func NewCA(cert *x509.Certificate, pool []*x509.Certificate, key crypto.PrivateKey) (CABundle, error) {
	if cert == nil {
		return nil, errors.Newf("cert must not be empty")
	}
	b := &bundle{
		cert: cert,
		pool: pool,
		key:  key,
	}
	if !cert.IsCA {
		return nil, errors.Newf("cert is not a ca")
	}
	err := b.init()
	if err != nil {
		return nil, errors.Wrap(err)
	}
	if key != nil {
		err := b.initTLS()
		if err != nil {
			return nil, errors.Wrap(err)
		}
	}
	return b, nil
}
