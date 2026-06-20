package certificate

import (
	"crypto/md5"
	"strings"

	"github.com/xhanio/errors"
	"github.com/xhanio/framingo/pkg/utils/certutil"
	"github.com/xhanio/framingo/pkg/utils/strutil"

	"github.com/xhanio/framingo/example/pkg/types/entity"
	"github.com/xhanio/framingo/example/pkg/types/orm"
)

func toEntity(ormCert *orm.Certificate) (*entity.Certificate, error) {
	var entityCert entity.Certificate
	entityCert.ID = ormCert.ID
	entityCert.Name = ormCert.Name
	entityCert.Comments = ormCert.Comments
	entityCert.Source = ormCert.Source
	entityCert.UploadType = ormCert.Type
	entityCert.IsCA = ormCert.IsCA
	entityCert.IsLocal = ormCert.IsLocal
	entityCert.RefCount = ormCert.RefCount

	// Decode bundle
	bundle, err := certutil.Decode(ormCert.CertBundle)
	if err != nil {
		return nil, errors.Wrapf(err, "Failed to decode cert bundle for cert %s", ormCert.Name)
	}
	entityCert.Bundle = bundle

	// Get cert details
	cert := bundle.Cert()
	if cert == nil {
		return nil, errors.Newf("failed to get cert from bundle")
	}

	entityCert.Version = cert.Version
	entityCert.SerialNumber = strutil.FormatHex(cert.SerialNumber, false)
	entityCert.Issuer = cert.Issuer.String()
	entityCert.Subject = cert.Subject.String()
	entityCert.StartsTime = cert.NotBefore.String()
	entityCert.ExpireTime = cert.NotAfter.String()

	// Calculate fingerprint
	fingerprintMD5 := entity.CertificateFingerprint{
		Algorithm: "MD5",
	}
	fingerprintSum := md5.Sum(cert.Raw)
	fingerprintMD5.Value = strings.ToUpper(strutil.FormatHex(fingerprintSum[:], false))
	entityCert.Fingerprint = fingerprintMD5

	// Extract and print extensions
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
	entityCert.Extensions = extensions
	return &entityCert, nil
}
