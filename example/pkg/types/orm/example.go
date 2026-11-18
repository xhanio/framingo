package orm

import "time"

type HelloWorld struct {
	ID        int64     `gorm:"primaryKey"`
	Message   string    `gorm:"type:text;not null"`
	CreatedAt time.Time `gorm:"autoCreateTime"`
	UpdatedAt time.Time `gorm:"autoUpdateTime"`
}

func (HelloWorld) TableName() string {
	return "helloworld_messages"
}
