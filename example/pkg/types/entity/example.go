package entity

import "time"

type Helloworld struct {
	ID        int64     `json:"id" gorm:"primaryKey"`
	Message   string    `json:"message" gorm:"type:text;not null"`
	CreatedAt time.Time `json:"created_at" gorm:"autoCreateTime"`
	UpdatedAt time.Time `json:"updated_at" gorm:"autoUpdateTime"`
}

// TableName specifies the table name for GORM
func (Helloworld) TableName() string {
	return "helloworld_messages"
}
