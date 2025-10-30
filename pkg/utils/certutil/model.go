package certutil

import (
	"crypto"
	"crypto/tls"
	"crypto/x509"
	"net"
	"time"

	"github.com/xhanio/framingo/pkg/types/common"
)

var (
	DefaultCommonName = "default"
)

type ClientRequest struct {
	CommonName  string
	ValidPeriod time.Duration
	KeepChain   bool
}

type ServerRequest struct {
	CommonName string
	DNSNames   []string
	IPs        []net.IP
	KeepChain  bool
}

type CARequest struct {
	CommonName string
	DNSNames   []string
	IPs        []net.IP
	KeepChain  bool
}

type Manager interface {
	CABundle
	ClientFiles(req *ClientRequest, certFile, keyFile string) error
	ServerFiles(req *ServerRequest, certFile, keyFile string) error
}

type CertBundle interface {
	CAs() []*x509.Certificate
	IsCA() bool
	Cert() *x509.Certificate
	CertDER() []byte
	CertPEM() []byte
	CertTLS() tls.Certificate
	Key() crypto.PrivateKey
	KeyDER() []byte
	KeyPEM() []byte
	Dump(certFile, keyFile string) error
	common.Debuggable
}

type CABundle interface {
	CertBundle
	SignClient(req *ClientRequest) (CertBundle, error)
	SignServer(req *ServerRequest) (CertBundle, error)
	SignCA(req *CARequest) (CABundle, error)
}
