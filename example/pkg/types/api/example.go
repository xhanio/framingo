package api

type CreateHelloWorldMessage struct {
	Message string `json:"message" form:"message" query:"message" validate:"required"`
}
