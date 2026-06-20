package orm

type Role struct {
	BaseModel
	Name        string `gorm:"column:name;type:character varying(250);not null"`
	Description string `gorm:"column:description;type:character varying(16000)"`
}

func (Role) TableName() string {
	return "roles"
}
