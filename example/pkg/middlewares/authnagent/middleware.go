package authnagent

import (
	"net/url"
	"path"
	"strconv"
	"strings"

	"github.com/labstack/echo/v4"
	"github.com/xhanio/errors"
	fapi "github.com/xhanio/framingo/pkg/types/api"
	"github.com/xhanio/framingo/pkg/types/common"
	"github.com/xhanio/framingo/pkg/utils/certutil"
	"github.com/xhanio/framingo/pkg/utils/log"
	"github.com/xhanio/framingo/pkg/utils/reflectutil"

	"github.com/xhanio/framingo/example/pkg/services/system/certificate"
	"github.com/xhanio/framingo/example/pkg/types/entity"
	"github.com/xhanio/framingo/example/pkg/types/preset"
)

var _ fapi.Middleware = (*middleware)(nil)

type middleware struct {
	log  log.Logger
	cert certificate.Manager
}

func New(cert certificate.Manager, opts ...Option) fapi.Middleware {
	m := &middleware{
		cert: cert,
	}
	for _, opt := range opts {
		opt(m)
	}
	if m.log == nil {
		m.log = log.Default
	}
	return m
}

func (m *middleware) Name() string {
	pkg, _ := reflectutil.Locate(m)
	return path.Base(pkg)
}

func (m *middleware) Dependencies() []common.Service {
	return []common.Service{m.cert}
}

func (m *middleware) Func(next echo.HandlerFunc) echo.HandlerFunc {
	return func(c echo.Context) error {
		ca, err := m.cert.DefaultCA()
		if err != nil {
			return errors.Wrap(err)
		}
		if c.Request().TLS != nil && len(c.Request().TLS.PeerCertificates) > 0 {
			m.log.Debugf("Request has certificates")
			// client cert from direct request
			for _, cert := range c.Request().TLS.PeerCertificates {
				if err := cert.CheckSignatureFrom(ca.Cert()); err != nil {
					m.log.Debugf("Error: Cert was not signed by ca")
					return errors.Unauthorized.Wrapf(err, "cert was not signed by ca")
				}
				// get agent id from cert common name
				commonName := cert.Subject.CommonName
				m.log.Debugf("cert common name: %s", commonName)
				parts := strings.Split(commonName, "=")
				if len(parts) != 2 || parts[0] != "CN" {
					m.log.Debugf("Certificate common name mismatched")
					return errors.Unauthorized.Newf("invalid CN format")
				}
				agentId, err := strconv.ParseInt(parts[1], 10, 32)
				if err != nil {
					m.log.Debugf("Error converting agent id to int32: %s", err.Error())
					return errors.Unauthorized.Newf("invalid CN Id format")
				}
				if agentId != 0 {
					m.log.Debugf("Agent id: %v", agentId)
					credential := &entity.Credential{
						Source:  preset.AuthSourceAgent,
						AgentID: parts[1],
					}
					c.Set(fapi.ContextKeyCredential, credential)
				}
			}
		} else {
			// client cert from nginx header
			escapedCert := c.Request().Header.Get(fapi.HeaderKeyClientCert)
			if len(escapedCert) == 0 {
				m.log.Debugf("Error: No cert present by agent")
				return errors.Unauthorized.Newf("no cert present by agent")
			}
			certStr, err := url.QueryUnescape(escapedCert)
			if err != nil {
				m.log.Debugf("Error: failed to unescape cert")
				return errors.Unauthorized.Wrapf(err, "failed to unescape cert")
			}
			cert, _, err := certutil.ParsePEMCert([]byte(certStr))
			if err != nil {
				m.log.Debugf("Error: invalid cert bytes")
				return errors.Unauthorized.Wrapf(err, "invalid cert bytes")
			}
			if err := cert.CheckSignatureFrom(ca.Cert()); err != nil {
				m.log.Debugf("Error: Cert not signed by CA")
				return errors.Unauthorized.Wrapf(err, "cert was not signed by ca")
			}
			// get agent id from cert common name
			commonName := cert.Subject.CommonName
			m.log.Debugf("cert common name: %s", commonName)
			parts := strings.Split(commonName, "=")
			if len(parts) != 2 || parts[0] != "CN" {
				m.log.Debugf("Certificate common name mismatched")
				return errors.Unauthorized.Newf("invalid CN format")
			}
			agentId, err := strconv.ParseInt(parts[1], 10, 32)
			if err != nil {
				m.log.Debugf("Error converting agent id to int32: %s", err.Error())
				return errors.Unauthorized.Newf("invalid CN Id format")
			}
			if agentId != 0 {
				m.log.Debugf("Agent id: %v", agentId)
				credential := &entity.Credential{
					Source:  preset.AuthSourceAgent,
					AgentID: parts[1],
				}
				c.Set(fapi.ContextKeyCredential, credential)
			}
		}
		return next(c)
	}
}
