package api

type CertificateUploadRequest struct {
	Type     string `json:"type" form:"type" validate:"required"`
	IsCA     bool   `json:"is_ca" form:"is_ca"`
	Name     string `json:"name" form:"name" validate:"required"`
	Password string `json:"password" form:"password"`
}

type CertificateUpdateRequest struct {
	Comments string `json:"comments"`
}

type CertificateGenerateRequest struct {
	Name           string `json:"name" validate:"required"`
	CommonName     string `json:"common_name" validate:"required"`
	SubjectAltName string `json:"subject_alt_name"`
	CAID           int32  `json:"ca_id" validate:"required"`
}

// CertificateSignRequest is POST /certificates/csr: the requester sends a CSR
// and keeps its private key; the manager's default CA signs it.
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
	CA            string `json:"ca"`          // PEM: what the requester trusts the manager by
}

type CertificateCreateResponse struct {
	CertificateID int32 `json:"certificate_id"`
}
