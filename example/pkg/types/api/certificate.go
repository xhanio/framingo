package api

type CertUploadBody struct {
	Type     string `json:"type" form:"type" validate:"required"`
	IsCA     bool   `json:"is_ca" form:"is_ca"`
	Name     string `json:"name" form:"name" validate:"required"`
	Password string `json:"password" form:"password"`
}

type CertUpdateBody struct {
	Comments string `json:"comments"`
}

type CertGenerateBody struct {
	Name           string `json:"name" validate:"required"`
	CommonName     string `json:"common_name" validate:"required"`
	SubjectAltName string `json:"subject_alt_name"`
	CAID           int32  `json:"ca_id" validate:"required"`
}

type CertUploadGenerateResponse struct {
	CertID int32 `json:"id"`
}
