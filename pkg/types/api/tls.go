package api

import (
	"crypto/tls"

	"github.com/xhanio/framingo/pkg/utils/certutil"
)

type ClientTLS struct {
	CertBundle certutil.CertBundle
	AuthType   tls.ClientAuthType
}

func (ct *ClientTLS) AsConfig() *tls.Config {
	result := &tls.Config{}
	if ct.CertBundle != nil {
		result.Certificates = []tls.Certificate{ct.CertBundle.CertTLS()}
		result.RootCAs = certutil.NewCertPool(ct.CertBundle.CAs()...)
	}
	result.ClientAuth = ct.AuthType
	if ct.AuthType == tls.NoClientCert {
		result.InsecureSkipVerify = true
	}
	return result
}

type ServerTLS struct {
	CertBundle  certutil.CertBundle
	AuthEnabled bool
}

func (st *ServerTLS) AsConfig() *tls.Config {
	result := &tls.Config{}
	if st.CertBundle != nil {
		result.Certificates = []tls.Certificate{st.CertBundle.CertTLS()}
		result.RootCAs = certutil.NewCertPool(st.CertBundle.CAs()...)
	}
	if !st.AuthEnabled || st.CertBundle == nil {
		result.InsecureSkipVerify = true
	}
	return result
}
