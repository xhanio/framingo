package orm

import form "github.com/xhanio/framingo/pkg/types/orm"

var _ form.Record[int32] = Role{}

type Role struct {
	BaseModel
	Name        string `gorm:"column:name;type:character varying(250);not null"`
	Description string `gorm:"column:description;type:character varying(16000)"`
}

func (Role) TableName() string {
	return "roles"
}
