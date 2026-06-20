package orm

type User struct {
	BaseModel
	Username             string `gorm:"column:username;type:character varying(250);not null"`
	Password             string `gorm:"column:password;type:character varying(60);not null"`
	OrganizationID       int32  `gorm:"column:organization_id;type:integer;not null"`
	Role                 string `gorm:"column:role;type:character varying(250);not null"`
	RequirePasswordReset bool   `gorm:"column:require_password_reset;type:boolean;not null"`
	FailedLoginsCount    int    `gorm:"column:failed_logins_count;type:integer;default:0;not null"`
	Expired              bool   `gorm:"column:expired;type:boolean;not null"`
	Locked               bool   `gorm:"column:locked;type:boolean;not null"`
	PassCanExpired       bool   `gorm:"column:pass_can_expired;type:boolean;not null"`
	Disabled             bool   `gorm:"column:disabled;type:boolean;not null"`
	ContactID            int32  `gorm:"column:contact_id;type:integer"`
}

func (User) TableName() string {
	return "users"
}
