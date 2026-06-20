package api

type UserCreateBody struct {
	Username       string `json:"username" validate:"required"`
	Password       string `json:"password" validate:"required"`
	Title          string `json:"title"`
	FirstName      string `json:"first_name"`
	LastName       string `json:"last_name"`
	Email          string `json:"email"`
	Role           string `json:"role" validate:"required"`
	ChangePassword bool   `json:"change_password"`
}

type UserCreateResponse struct {
	UserID int32 `json:"user_id"`
}

type UserUpdateBody struct {
	Username       *string `json:"username,omitempty"`
	Title          *string `json:"title,omitempty"`
	FirstName      *string `json:"first_name,omitempty"`
	LastName       *string `json:"last_name,omitempty"`
	Email          *string `json:"email,omitempty"`
	Role           *string `json:"role,omitempty"`
	ChangePassword bool    `json:"change_password"`
}

type UserResetPasswordBody struct {
	Password    string `json:"password" validate:"required"`
	OldPassword string `json:"old_password"`
}
