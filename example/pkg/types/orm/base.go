package orm

// BaseModel's getters plus a TableName make a table a form.Record[int32]; a table that is one says so with a var _.
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
