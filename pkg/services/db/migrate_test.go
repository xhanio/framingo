package db_test

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/spf13/viper"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/xhanio/framingo/pkg/services/db"
	_ "github.com/xhanio/framingo/pkg/services/db/drivers/sqlite"
	"github.com/xhanio/framingo/pkg/utils/confutil"
)

// writeMigrations lays out golang-migrate's file naming convention in a temp
// directory and returns it.
func writeMigrations(t *testing.T, files map[string]string) string {
	t.Helper()
	dir := t.TempDir()
	for name, body := range files {
		require.NoError(t, os.WriteFile(filepath.Join(dir, name), []byte(body), 0o600))
	}
	return dir
}

// newMigratedMgr builds a file-backed SQLite manager that migrates from sqlDir
// on Init. A file (rather than :memory:) is used so every pooled connection
// observes the same schema.
func newMigratedMgr(t *testing.T, sqlDir string, version uint) (db.Manager, error) {
	t.Helper()

	v := viper.New()
	v.Set("db.connection.max_open", 1)
	v.Set("db.connection.max_idle", 1)
	ctx := confutil.WrapContext(context.Background(), v)

	mgr := db.New(
		db.WithType(db.SQLite),
		db.WithDataSource(db.Source{DBName: filepath.Join(t.TempDir(), "test.db")}),
		db.WithMigration(sqlDir, version),
	)
	return mgr, mgr.Init(ctx)
}

// TestMigrate_AppliesUpMigrations covers the vendored golang-migrate driver
// end to end: it must create the version table, run the migrations, and record
// the resulting version.
func TestMigrate_AppliesUpMigrations(t *testing.T) {
	dir := writeMigrations(t, map[string]string{
		"0001_create_users.up.sql":   `CREATE TABLE users (id INTEGER PRIMARY KEY, name TEXT NOT NULL);`,
		"0001_create_users.down.sql": `DROP TABLE users;`,
		"0002_add_email.up.sql":      `ALTER TABLE users ADD COLUMN email TEXT;`,
		"0002_add_email.down.sql":    `ALTER TABLE users DROP COLUMN email;`,
	})

	mgr, err := newMigratedMgr(t, dir, 0)
	require.NoError(t, err)

	// Both migrations applied: the v2 column must exist.
	require.NoError(t, mgr.ORM().Exec(`INSERT INTO users (id, name, email) VALUES (1, 'ada', 'ada@example.com')`).Error)

	var name string
	require.NoError(t, mgr.ORM().Raw(`SELECT name FROM users WHERE id = 1`).Scan(&name).Error)
	assert.Equal(t, "ada", name)

	// The vendored driver tracks state in golang-migrate's canonical table.
	var version int
	var dirty bool
	require.NoError(t, mgr.ORM().Raw(`SELECT version, dirty FROM schema_migrations LIMIT 1`).Row().Scan(&version, &dirty))
	assert.Equal(t, 2, version)
	assert.False(t, dirty, "migration should not leave the schema dirty")
}

// TestMigrate_StopsAtRequestedVersion pins migration to v1, so the v2 column
// must not exist.
func TestMigrate_StopsAtRequestedVersion(t *testing.T) {
	dir := writeMigrations(t, map[string]string{
		"0001_create_users.up.sql":   `CREATE TABLE users (id INTEGER PRIMARY KEY, name TEXT NOT NULL);`,
		"0001_create_users.down.sql": `DROP TABLE users;`,
		"0002_add_email.up.sql":      `ALTER TABLE users ADD COLUMN email TEXT;`,
		"0002_add_email.down.sql":    `ALTER TABLE users DROP COLUMN email;`,
	})

	mgr, err := newMigratedMgr(t, dir, 1)
	require.NoError(t, err)

	var version int
	require.NoError(t, mgr.ORM().Raw(`SELECT version FROM schema_migrations LIMIT 1`).Scan(&version).Error)
	assert.Equal(t, 1, version)

	err = mgr.ORM().Exec(`INSERT INTO users (id, name, email) VALUES (1, 'ada', 'a@b.c')`).Error
	assert.Error(t, err, "email column should not exist at version 1")
}

// TestMigrate_ReportsFailure ensures a broken migration surfaces as an error
// rather than being silently swallowed.
func TestMigrate_ReportsFailure(t *testing.T) {
	dir := writeMigrations(t, map[string]string{
		"0001_broken.up.sql":   `THIS IS NOT VALID SQL;`,
		"0001_broken.down.sql": `SELECT 1;`,
	})

	_, err := newMigratedMgr(t, dir, 0)
	require.Error(t, err, "invalid migration SQL must fail Init")
}
