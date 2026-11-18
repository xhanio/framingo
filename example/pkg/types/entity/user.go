package entity

type User struct {
	ID             int32  `json:"id"`
	Username       string `json:"username"`
	Title          string `json:"title"`
	FirstName      string `json:"first_name"`
	LastName       string `json:"last_name"`
	Email          string `json:"email"`
	Role           string `json:"role"`
	ChangePassword bool   `json:"change_password"`
	// for building a credential
	OrganizationID       int32 `json:"-"`
	RequirePasswordReset bool  `json:"-"`
}

type UserCreateOptions struct {
	OrganizationID       int32
	Username             string
	FirstName            string
	LastName             string
	Email                string
	Password             string
	Title                string //Optional Param
	Role                 string
	RestAPI              bool
	ChangePassword       bool
	RequirePasswordReset bool
}

type UserListOptions struct {
	OrganizationID int32
	// constrained: an unknown column is a 500 that leaks the schema, and password is a column too
	SortBy string `validate:"omitempty,oneof=id username role"`
	Desc   bool
}

// UserUpdateOptions: a nil field is left as it is; the repository writes only the fields present.
type UserUpdateOptions struct {
	Username  *string `json:"username,omitempty"`
	FirstName *string `json:"first_name,omitempty"`
	LastName  *string `json:"last_name,omitempty"`
	Email     *string `json:"email,omitempty"`
	Title     *string `json:"title,omitempty"` //Optional Param
	Role      *string `json:"role,omitempty"`
	// Expired forces a password change at next login.
	Expired *bool `json:"expired,omitempty"`
}

type UserChangePasswordOptions struct {
	OldPassword string
	Password    string
}

type UserResetPasswordOptions struct {
	Password string
}
