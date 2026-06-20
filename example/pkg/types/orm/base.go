package orm

import (
	"encoding/json"
	"fmt"
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
	ID     int32 `gorm:"primaryKey;column:id;type:integer;not null"`
	Erased bool  `gorm:"column:erased;type:boolean;not null;default false"`
	Hidden bool  `gorm:"column:hidden;type:boolean"`
	// UpdateTime time.Time `gorm:"column:update_time;type:timestamp with time zone"`
	Version int64 `gorm:"column:version;type:bigint"`
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

/*
* Leverage PostgreSQL's jsonb_build capabilities to directly generate the JSON structure required by the frontend within SQL,
* bypassing the ORM process to avoid impedance mismatch issues associated with object-relational mapping
 */
type JsonAgg struct {
	Values json.RawMessage `grom:"column:values;type:jsonb"`
}
