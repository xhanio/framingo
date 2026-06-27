package db

import (
	"database/sql"
	"sync"

	"github.com/golang-migrate/migrate/v4/database"
	"github.com/xhanio/errors"
	"gorm.io/gorm"
)

// Driver bundles the per-database-engine hooks needed by the manager.
// Each driver subpackage under pkg/services/db/drivers/* registers a Driver
// during its init() so the core package never imports concrete GORM or
// golang-migrate drivers.
type Driver struct {
	Dialector func(dsn string) gorm.Dialector
	Migration func(sqlDB *sql.DB) (database.Driver, error)
	DSN       func(s Source) (string, error)
	Cleanup   func(db *gorm.DB, dbName string, schema bool) error
}

type registry struct {
	mu      sync.RWMutex
	drivers map[string]Driver
}

var reg = &registry{drivers: map[string]Driver{}}

func (r *registry) register(dbtype string, d Driver) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if _, exists := r.drivers[dbtype]; exists {
		panic("db: driver already registered: " + dbtype)
	}
	r.drivers[dbtype] = d
}

func (r *registry) lookup(dbtype string) (Driver, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	d, ok := r.drivers[dbtype]
	if !ok {
		return Driver{}, errors.Newf("unsupported db type: %s (driver not registered — blank-import the corresponding pkg/services/db/drivers/* package)", dbtype)
	}
	return d, nil
}

// Register makes a Driver available under the given dbtype name. It is meant
// to be called from the init() of a driver subpackage; double-registration
// panics so misconfiguration surfaces at startup.
func Register(dbtype string, d Driver) {
	reg.register(dbtype, d)
}

func lookupDriver(dbtype string) (Driver, error) {
	return reg.lookup(dbtype)
}
