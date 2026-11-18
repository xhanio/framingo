package certificate

import (
	"crypto/md5"
	"strings"

	"github.com/xhanio/errors"
	"github.com/xhanio/framingo/pkg/utils/certutil"
	"github.com/xhanio/framingo/pkg/utils/strutil"

	"github.com/xhanio/framingo/example/pkg/types/entity"
)

// describe derives the parsed fields from the decoded bundle; derivation, not storage, so it lives here.
func describe(c *entity.Certificate) (*entity.Certificate, error) {
	if c.Bundle == nil {
		return nil, errors.Newf("certificate %s holds no bundle", c.Name)
	}
	cert := c.Bundle.Cert()
	if cert == nil {
		return nil, errors.Newf("failed to get cert from bundle of %s", c.Name)
	}
	c.Version = cert.Version
	c.SerialNumber = strutil.FormatHex(cert.SerialNumber, false)
	c.Issuer = cert.Issuer.String()
	c.Subject = cert.Subject.String()
	c.StartsTime = cert.NotBefore.String()
	c.ExpireTime = cert.NotAfter.String()

	fingerprint := md5.Sum(cert.Raw)
	c.Fingerprint = entity.CertificateFingerprint{
		Algorithm: "MD5",
		Value:     strings.ToUpper(strutil.FormatHex(fingerprint[:], false)),
	}

	c.Extensions = nil
	for _, ext := range cert.Extensions {
		extID := ext.Id.String()
		value, err := certutil.GetExtensionValue(cert, extID)
		if err != nil {
			continue
		}
		c.Extensions = append(c.Extensions, entity.CertificateExtension{
			Name:  certutil.OIDStringToNameMap[extID],
			Value: value,
		})
	}
	return c, nil
}
