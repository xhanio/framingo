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

type CertificateCreateResponse struct {
	CertificateID int32 `json:"certificate_id"`
}
