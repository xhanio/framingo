package orm

type Organization struct {
	BaseModel
	Name string `gorm:"column:name;not null;type:varchar(100)"`
}

func (Organization) TableName() string {
	return "organizations"
}
