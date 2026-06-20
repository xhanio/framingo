package entity

import "github.com/xhanio/framingo/pkg/utils/certutil"

const (
	CertTypePKCS12     = "PKCS#12"
	CertTypeRemoteCert = "RemoteCertificate" // cert only
	CertTypeCertKey    = "Certificate"       // cert and key

	CertSourceUser    = "user"
	CertSourceFactory = "factory"

	CertCategoryLocalCA           = "localCA"
	CertCategoryRemoteCA          = "remoteCA"
	CertCategoryLocalCertificate  = "localCertificate"
	CertCategoryRemoteCertificate = "remoteCertificate"
)

type Certificate struct {
	ID           int32                  `json:"id"`
	Name         string                 `json:"name"`
	IsLocal      bool                   `json:"is_local"`
	IsCA         bool                   `json:"is_ca"`
	Comments     string                 `json:"comments"`
	Version      int                    `json:"version"`
	Source       string                 `json:"source"`
	SerialNumber string                 `json:"serial_number"`
	Subject      string                 `json:"subject"`
	Issuer       string                 `json:"issuer"`
	StartsTime   string                 `json:"start_time"`
	ExpireTime   string                 `json:"expire_time"`
	Fingerprint  CertificateFingerprint `json:"fingerprint"`
	Extensions   []CertificateExtension `json:"extensions"`
	UploadType   string                 `json:"-"`
	Bundle       certutil.CertBundle    `json:"-"`
	RefCount     int                    `json:"-"`
}

type CertificateFingerprint struct {
	Algorithm string `json:"algorithm"`
	Value     string `json:"value"`
}

type CertificateExtension struct {
	Name  string `json:"name"`
	Value string `json:"value"`
}

type CertCreateOptions struct {
	Name     string
	IsCA     bool
	IsLocal  bool
	Type     string
	Bundle   certutil.CertBundle
	Comments string
	Source   string
}

type CertIssueOptions struct {
	Name           string
	CommonName     string
	SubjectAltName string
	CAID           int32
}

type CertUpdateOptions struct {
	Comments string
}

type CertUpdateBundleOptions struct {
	Bundle []byte
}

type CertListOptions struct {
	IsCA    *bool //If nil, return all certificates; if true, return only CA certificates; if false, return only non-CA certificates
	IsLocal bool  // filter only certificates with key stored locally
}
