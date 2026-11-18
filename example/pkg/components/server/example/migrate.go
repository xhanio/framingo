package example

import (
	"context"

	"github.com/xhanio/errors"

	"github.com/xhanio/framingo/pkg/utils/confutil"
)

// Migrate is the one-off schema run (the prod Job) so replicas never race over
// it; db.migration.version 0 means latest.
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
	// db reads its settings from the config in ctx, as under the supervisor
	ctx = confutil.WrapContext(ctx, confutil.New(m.config))
	if err := database.Init(ctx); err != nil {
		return errors.Wrapf(err, "failed to migrate the database")
	}
	m.log.Info("database migrated")
	return nil
}
