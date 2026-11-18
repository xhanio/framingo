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

func writeMigrations(t *testing.T, files map[string]string) string {
	t.Helper()
	dir := t.TempDir()
	for name, body := range files {
		require.NoError(t, os.WriteFile(filepath.Join(dir, name), []byte(body), 0o600))
	}
	return dir
}

// file-backed, not :memory:, so every pooled connection sees the same schema
func newMigratedMgr(t *testing.T, sqlDir string, version uint) (db.Manager, error) {
	t.Helper()

	v := viper.New()
	v.Set("db.connection.max_open", 1)
	v.Set("db.connection.max_idle", 1)
	ctx := confutil.WrapContext(context.Background(), confutil.New(v))

	mgr := db.New(
		db.WithType(db.SQLite),
		db.WithDataSource(db.Source{DBName: filepath.Join(t.TempDir(), "test.db")}),
		db.WithMigration(sqlDir, version),
	)
	return mgr, mgr.Init(ctx)
}

func TestMigrate_AppliesUpMigrations(t *testing.T) {
	dir := writeMigrations(t, map[string]string{
		"0001_create_users.up.sql":   `CREATE TABLE users (id INTEGER PRIMARY KEY, name TEXT NOT NULL);`,
		"0001_create_users.down.sql": `DROP TABLE users;`,
		"0002_add_email.up.sql":      `ALTER TABLE users ADD COLUMN email TEXT;`,
		"0002_add_email.down.sql":    `ALTER TABLE users DROP COLUMN email;`,
	})

	mgr, err := newMigratedMgr(t, dir, 0)
	require.NoError(t, err)

	require.NoError(t, mgr.ORM().Exec(`INSERT INTO users (id, name, email) VALUES (1, 'ada', 'ada@example.com')`).Error)

	var name string
	require.NoError(t, mgr.ORM().Raw(`SELECT name FROM users WHERE id = 1`).Scan(&name).Error)
	assert.Equal(t, "ada", name)

	var version int
	var dirty bool
	require.NoError(t, mgr.ORM().Raw(`SELECT version, dirty FROM schema_migrations LIMIT 1`).Row().Scan(&version, &dirty))
	assert.Equal(t, 2, version)
	assert.False(t, dirty, "migration should not leave the schema dirty")
}

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

func TestMigrate_ReportsFailure(t *testing.T) {
	dir := writeMigrations(t, map[string]string{
		"0001_broken.up.sql":   `THIS IS NOT VALID SQL;`,
		"0001_broken.down.sql": `SELECT 1;`,
	})

	_, err := newMigratedMgr(t, dir, 0)
	require.Error(t, err, "invalid migration SQL must fail Init")
}
