package entity

import (
	"time"

	"github.com/xhanio/framingo/pkg/utils/certutil"
)

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

	CertUsageClient = "client"
	CertUsageServer = "server"
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

// CertSignOptions signs a requester's CSR with the default CA - the one agents
// are verified against. The requester keeps its private key; CommonName and
// SubjectAltName are what the CA grants, and the CSR may ask for no other name.
type CertSignOptions struct {
	Name           string
	CSR            []byte // PEM or DER
	CommonName     string
	SubjectAltName string
	Usage          string        // CertUsageClient (empty) or CertUsageServer
	ValidPeriod    time.Duration // 0: the service's default
}

type CertUpdateOptions struct {
	Comments string
}

type CertListOptions struct {
	IsCA    *bool // nil: all; true: CA only; false: non-CA only
	IsLocal bool  // filter only certificates with key stored locally
}
