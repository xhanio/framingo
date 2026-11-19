package api

import (
	"crypto/tls"
	"crypto/x509"

	"github.com/xhanio/framingo/pkg/utils/certutil"
)

// ClientTLS is the certificate a client presents, if any, and the CAs it
// verifies the server against - the system's when RootCAs is empty.
type ClientTLS struct {
	CertBundle certutil.CertBundle
	RootCAs    []*x509.Certificate
}

func (ct *ClientTLS) AsConfig() *tls.Config {
	result := &tls.Config{}
	if ct == nil {
		return result
	}
	if ct.CertBundle != nil {
		result.Certificates = []tls.Certificate{ct.CertBundle.CertTLS()}
	}
	if len(ct.RootCAs) > 0 {
		result.RootCAs = certutil.NewCertPool(ct.RootCAs...)
	}
	return result
}

// ServerTLS is the server's certificate and the CAs a client certificate is
// verified against. One is asked for, not required: a listener serves users
// by session as well, and the route decides who must present one.
type ServerTLS struct {
	CertBundle certutil.CertBundle
	ClientCAs  []*x509.Certificate // empty: no client certificate is asked for
}

func (st *ServerTLS) AsConfig() *tls.Config {
	result := &tls.Config{}
	if st.CertBundle != nil {
		result.Certificates = []tls.Certificate{st.CertBundle.CertTLS()}
	}
	if len(st.ClientCAs) > 0 {
		result.ClientAuth = tls.VerifyClientCertIfGiven
		result.ClientCAs = certutil.NewCertPool(st.ClientCAs...)
	}
	return result
}
