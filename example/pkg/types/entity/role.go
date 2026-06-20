package entity

type Role struct {
	ID          int32  `json:"id"`
	Name        string `json:"name"`
	Description string `json:"description"`
}

type RoleCreateOptions struct {
	Name        string
	Description string //Optional Param
}

type RoleUpdateOptions struct {
	Name        *string `json:"name,omitempty"`
	Description *string `json:"description,omitempty"`
}

type PermissionSetOptions struct {
	Permissions []string
}
