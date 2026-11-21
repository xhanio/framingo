// the db service owns the pool
package db

import "gorm.io/gorm"

type pool struct {
	orm *gorm.DB
}
