package orm

import "time"

type Helloworld struct {
	ID        int64     `gorm:"primaryKey"`
	Message   string    `gorm:"type:text;not null"`
	CreatedAt time.Time `gorm:"autoCreateTime"`
	UpdatedAt time.Time `gorm:"autoUpdateTime"`
}

// TableName specifies the table name for GORM
func (Helloworld) TableName() string {
	return "helloworld_messages"
}
