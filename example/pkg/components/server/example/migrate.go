package example

import (
	"context"

	"github.com/xhanio/errors"

	"github.com/xhanio/framingo/pkg/utils/confutil"
)

// Migrate brings the database schema to db.migration.version (0 for the
// latest) from the migrations in db.migration.dir, and returns. It is the
// one-off run a deployment gives migrating to - the Kubernetes Job in prod -
// so the daemons, however many replicas, never race each other over the
// schema. It reads the same config, environment included, as the daemon:
// FRAMINGO_EXAMPLE_DB_MIGRATION_DIR names the directory where the config
// file leaves it out.
func Migrate(ctx context.Context, configPath string) error {
	m := &manager{config: newConfig(configPath)}
	if err := m.initConfig(); err != nil {
		return errors.Wrap(err)
	}
	if m.config.GetString("db.migration.dir") == "" {
		return errors.InvalidArgument.Newf("db.migration.dir is not set: no migrations to apply")
	}
	m.log = m.newLogger()
	database := m.newDatabase()
	// the database reads its connection settings from the config in ctx, as
	// the supervisor hands it over to the daemon's
	ctx = confutil.WrapContext(ctx, confutil.New(m.config))
	if err := database.Init(ctx); err != nil {
		return errors.Wrapf(err, "failed to migrate the database")
	}
	m.log.Info("database migrated")
	return nil
}
