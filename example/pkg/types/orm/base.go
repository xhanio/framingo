package orm

// BaseModel carries the columns most tables here share - id, erased, hidden,
// version. Its getters, with a table's own TableName, make an embedding table
// a framework form.Record[int32], and such a table says so with a var _ beside
// it. Embedding it is a convenience, not a contract: a table whose columns
// differ, like helloworld_messages, defines its own and is no Record.
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
