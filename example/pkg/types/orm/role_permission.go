package orm

import form "github.com/xhanio/framingo/pkg/types/orm"

var _ form.Record[int32] = RolePermission{}

type RolePermission struct {
	BaseModel
	RoleID     int32  `gorm:"column:role_id;;type:integer;not null"`
	Permission string `gorm:"column:permission;type:character varying(250);not null"`
}

func (RolePermission) TableName() string {
	return "role_permissions"
}
