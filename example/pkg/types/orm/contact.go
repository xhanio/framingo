package orm

type Contact struct {
	BaseModel
	Email          string `gorm:"column:email;type:varchar(500)"`
	FirstName      string `gorm:"column:first_name;type:varchar(50)"`
	LastName       string `gorm:"column:last_name;type:varchar(50)"`
	OrganizationID int32  `gorm:"column:organization_id;type:integer"`
	Title          string `gorm:"column:title;type:varchar(50)"`
}

func (Contact) TableName() string {
	return "contacts"
}
