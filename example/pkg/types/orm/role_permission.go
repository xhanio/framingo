package orm

type RolePermission struct {
	BaseModel
	RoleID     int32  `gorm:"column:role_id;;type:integer;not null"`
	Permission string `gorm:"column:permission;type:character varying(250);not null"`
}

func (RolePermission) TableName() string {
	return "role_permissions"
}
