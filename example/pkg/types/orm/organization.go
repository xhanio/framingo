package orm

import form "github.com/xhanio/framingo/pkg/types/orm"

var _ form.Record[int32] = Organization{}

type Organization struct {
	BaseModel
	Name string `gorm:"column:name;not null;type:varchar(100)"`
}

func (Organization) TableName() string {
	return "organizations"
}
