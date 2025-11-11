package entity

import "time"

type Helloworld struct {
	ID        int64     `json:"id"`
	Message   string    `json:"message"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}
