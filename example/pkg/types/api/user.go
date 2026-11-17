package api

type UserCreateRequest struct {
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

// UserProfileUpdateRequest is PATCH /user: the caller's own contact details.
// A username or role in the body has no field to land in, so the caller
// cannot change who they are through it.
type UserProfileUpdateRequest struct {
	Title     *string `json:"title,omitempty"`
	FirstName *string `json:"first_name,omitempty"`
	LastName  *string `json:"last_name,omitempty"`
	Email     *string `json:"email,omitempty"`
}

// UserUpdateRequest is PATCH /users/:id. A field left out is left as it is -
// change_password included, hence the pointer.
type UserUpdateRequest struct {
	Username       *string `json:"username,omitempty"`
	Title          *string `json:"title,omitempty"`
	FirstName      *string `json:"first_name,omitempty"`
	LastName       *string `json:"last_name,omitempty"`
	Email          *string `json:"email,omitempty"`
	Role           *string `json:"role,omitempty"`
	ChangePassword bool    `json:"change_password"`
}

type UserResetPasswordRequest struct {
	Password    string `json:"password" validate:"required"`
	OldPassword string `json:"old_password"`
}
