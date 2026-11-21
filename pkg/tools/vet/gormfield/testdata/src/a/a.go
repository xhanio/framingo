package a

import "gorm.io/gorm"

type Database interface {
	FromContext() *gorm.DB
}

type keeps struct {
	db *gorm.DB // want `a \*gorm.DB kept in a field`
}

// the handle is kept, and asked each time
type asks struct {
	db Database
}

func (m *asks) query() {
	tx := m.db.FromContext()
	_ = tx
}

// a parameter and a local are not kept
func helper(tx *gorm.DB) *gorm.DB {
	local := tx
	return local
}

type excused struct {
	//framingo:ignore gormfield a one-shot migration tool, gone before any restart
	db *gorm.DB
}
