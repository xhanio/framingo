package orm

import (
	"fmt"
	"time"
)

const NoVersion int64 = 0

type Record interface {
	GetID() int32
	GetErased() bool
	GetVersion() int64
	TableName() string
}

type Referenced interface {
	References() []Reference
}

type BaseModel struct {
	ID        int32     `gorm:"primaryKey;autoIncrement;column:id;type:integer;not null"`
	Erased    bool      `gorm:"column:erased;type:boolean;not null;default false"`
	Hidden    bool      `gorm:"column:hidden;type:boolean"`
	CreatedAt time.Time `gorm:"column:created_at;type:timestamp"`
	UpdatedAt time.Time `gorm:"column:updated_at;type:timestamp"`
	Version   int64     `gorm:"column:version;type:bigint"`
}

func (b BaseModel) GetID() int32 {
	return b.ID
}

func (b BaseModel) GetErased() bool {
	return b.Erased
}

func (b BaseModel) GetVersion() int64 {
	return b.Version
}

type Reference struct {
	TableName string
	ID        int32
}

func (r Reference) Key() string {
	return fmt.Sprintf("%s/%d", r.TableName, r.ID)
}
