package api

type HelloWorldCreateRequest struct {
	Message string `json:"message" form:"message" query:"message" validate:"required"`
}
