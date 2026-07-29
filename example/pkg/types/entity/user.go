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
	// SortBy reaches GORM as a column identifier. GORM quotes it, so it is not
	// injectable, but an unknown value is a runtime SQL error (a 500) that also
	// answers "does this column exist?". Constrain it to the columns a caller
	// may legitimately order by — password is a column too.
	SortBy string `validate:"omitempty,oneof=id username role"`
	Desc   bool   // false for ascending order; true for descending order
}

// Pointer fields to differentiate between an empty string and a nil case
type UserUpdateOptions struct {
	Username  *string `json:"username,omitempty"`
	FirstName *string `json:"first_name,omitempty"`
	LastName  *string `json:"last_name,omitempty"`
	Email     *string `json:"email,omitempty"`
	Title     *string `json:"title,omitempty"` //Optional Param
	Role      *string `json:"role,omitempty"`
	Expired   bool    `json:"expired"`
}

type UserResetPasswordOptions struct {
	Password    string
	OldPassword string
}
