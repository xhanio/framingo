package certificate

import (
	"crypto/md5"
	"strings"

	"github.com/xhanio/errors"
	"github.com/xhanio/framingo/pkg/utils/certutil"
	"github.com/xhanio/framingo/pkg/utils/strutil"

	"github.com/xhanio/framingo/example/pkg/types/entity"
)

// describe fills in what is read out of the certificate in c's bundle -
// version, serial, issuer, subject, validity, fingerprint and extensions. The
// repository hands the bundle over decoded; these are worked out from it.
func describe(c *entity.Certificate) error {
	if c.Bundle == nil {
		return errors.Newf("certificate %s has no bundle", c.Name)
	}
	cert := c.Bundle.Cert()
	if cert == nil {
		return errors.Newf("failed to get cert from bundle")
	}

	c.Version = cert.Version
	c.SerialNumber = strutil.FormatHex(cert.SerialNumber, false)
	c.Issuer = cert.Issuer.String()
	c.Subject = cert.Subject.String()
	c.StartsTime = cert.NotBefore.String()
	c.ExpireTime = cert.NotAfter.String()

	// Calculate fingerprint
	fingerprintSum := md5.Sum(cert.Raw)
	c.Fingerprint = entity.CertificateFingerprint{
		Algorithm: "MD5",
		Value:     strings.ToUpper(strutil.FormatHex(fingerprintSum[:], false)),
	}

	// Extract the extensions
	var extensions []entity.CertificateExtension
	for _, ext := range cert.Extensions {
		extID := ext.Id.String()
		extName := certutil.OIDStringToNameMap[extID]
		extValue, err := certutil.GetExtensionValue(cert, extID)
		if err != nil {
			continue
		}
		extensions = append(extensions, entity.CertificateExtension{Name: extName, Value: extValue})
	}
	c.Extensions = extensions
	return nil
}
