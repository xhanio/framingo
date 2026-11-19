package api

type CertificateUploadRequest struct {
	Type     string `json:"type" form:"type" validate:"required"`
	IsCA     bool   `json:"is_ca" form:"is_ca"`
	Name     string `json:"name" form:"name" validate:"required"`
	Password string `json:"password" form:"password"`
}

// The formats a certificate downloads as, whatever it was uploaded as.
const (
	CertFormatPEM = "pem" // the certificate alone
	CertFormatTar = "tar" // the certificate and its key, as <name>.crt and <name>.key
	CertFormatP12 = "p12" // the certificate, its key and chain, encrypted with Password
)

// CertificateDownloadRequest is POST /certificates/:id/download. No format is
// tar for a certificate with its key, pem for one without.
type CertificateDownloadRequest struct {
	Format   string `json:"format" validate:"omitempty,oneof=pem tar p12"`
	Password string `json:"password" validate:"required_if=Format p12"` // the caller's: nothing stores it
}

type CertificateUpdateRequest struct {
	Comments string `json:"comments"`
}

type CertificateGenerateRequest struct {
	Name           string `json:"name" validate:"required"`
	CommonName     string `json:"common_name" validate:"required"`
	SubjectAltName string `json:"subject_alt_name"`
	CAID           int32  `json:"ca_id"` // empty: the server CA
}

// CertificateSignRequest is POST /certificates/csr: the requester sends a CSR
// and keeps its private key; the manager's agent CA signs it.
type CertificateSignRequest struct {
	Name           string `json:"name" validate:"required"`
	CSR            string `json:"csr" validate:"required"` // PEM
	CommonName     string `json:"common_name" validate:"required"`
	SubjectAltName string `json:"subject_alt_name"`
	Usage          string `json:"usage" validate:"omitempty,oneof=client server"`
	ValidPeriod    string `json:"valid_period"` // a Go duration, e.g. 720h; empty is the default
}

type CertificateSignResponse struct {
	CertificateID int32  `json:"certificate_id"`
	Certificate   string `json:"certificate"` // PEM
	ServerCA      string `json:"server_ca"`   // PEM: what the requester trusts the manager by
}

// CertificateCAResponse is GET /certificates/ca: the system CAs' certificates,
// for whatever verifies against them - nginx checking agents, a trust store.
type CertificateCAResponse struct {
	ServerCA string `json:"server_ca"` // PEM
	AgentCA  string `json:"agent_ca"`  // PEM
}

type CertificateCreateResponse struct {
	CertificateID int32 `json:"certificate_id"`
}
