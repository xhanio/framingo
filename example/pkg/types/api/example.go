package api

type CreateHelloworldMessage struct {
	Message string `json:"message" form:"message" query:"message" validate:"required"`
}
