package entity

type Organization struct {
	ID   int32  `json:"id"`
	Name string `json:"name"`
}

type OrganizationCreateOptions struct {
	Name string
}

type OrganizationUpdateOptions struct {
	Name *string `json:"name,omitempty"`
}
