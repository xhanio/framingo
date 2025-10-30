package client

import (
	"crypto/tls"
	"net/http"
	"time"

	"github.com/xhanio/framingo/pkg/types/common"
	"github.com/xhanio/framingo/pkg/types/common/api"
	"github.com/xhanio/framingo/pkg/utils/certutil"
	"github.com/xhanio/framingo/pkg/utils/log"
)

type Option func(*client)

func WithLogger(logger log.Logger) Option {
	return func(c *client) {
		c.log = logger
	}
}

func WithCert(cert certutil.CertBundle, authType tls.ClientAuthType) Option {
	return func(c *client) {
		c.tlsConfig = &api.ClientTLS{
			CertBundle: cert,
			AuthType:   authType,
		}
	}
}

// func WithCertPEM(caPEM, certPEM, keyPEM []byte, authEnabled bool) Option {
// 	return func(c *client) {
// 		c.tlsConfig = &api.TLSConfig{
// 			CA:                caPEM,
// 			Cert:              certPEM,
// 			Key:               keyPEM,
// 			ServerAuthEnabled: authEnabled,
// 		}
// 	}
// }

// func WithCertFile(caFile, certFile, keyFile string, authEnabled bool) Option {
// 	return func(c *client) {
// 		c.tlsConfig = &api.TLSConfig{
// 			CAFile:            caFile,
// 			CertFile:          certFile,
// 			KeyFile:           keyFile,
// 			ServerAuthEnabled: authEnabled,
// 		}
// 	}
// }

func WithDebug() Option {
	return func(c *client) {
		c.debug = true
	}
}

func WithTimeout(timeout time.Duration) Option {
	return func(c *client) {
		c.timeout = timeout
	}
}

type RequestOption func(*Request)

func WithRequestHeaders(headers ...common.Pair[string, string]) RequestOption {
	return func(r *Request) {
		r.Headers = append(r.Headers, headers...)
	}
}

func WithRequestCookies(cookies ...*http.Cookie) RequestOption {
	return func(r *Request) {
		r.Cookies = append(r.Cookies, cookies...)
	}
}
