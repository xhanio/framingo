package example

import (
	"context"
	"path/filepath"
	"strings"
	"time"

	"github.com/spf13/viper"
	"github.com/xhanio/errors"
	"github.com/xhanio/framingo/pkg/types/info"
	"github.com/xhanio/framingo/pkg/utils/envutil"

	"github.com/xhanio/framingo/example/pkg/utils/infra"
)

func newConfig(configPath string) *viper.Viper {
	conf := viper.New()
	conf.SetConfigFile(configPath)
	infra.EnvPrefix = envutil.EnvPrefix(info.ProductName)
	conf.SetEnvPrefix(infra.EnvPrefix)
	// db.source.host reads FRAMINGO_EXAMPLE_DB_SOURCE_HOST (DB_SOURCE_HOST without gopro's product name)
	conf.SetEnvKeyReplacer(strings.NewReplacer(".", "_"))
	conf.AutomaticEnv()
	return conf
}

func (m *manager) initConfig() error {
	infra.StartTime = time.Now()
	configFile := m.config.ConfigFileUsed()
	if err := m.config.ReadInConfig(); err != nil {
		return errors.Wrapf(err, "failed to read config file %s", configFile)
	}
	absPath, err := filepath.Abs(configFile)
	if err != nil {
		return errors.Wrapf(err, "failed to resolve config path %s", configFile)
	}
	infra.ConfigDir = filepath.Dir(absPath)
	return nil
}

// reload is SIGHUP's, the one time the file is re-read: services stopped
// first, no Init reads the config while it is rewritten, and every service
// starts on one version of the file. A file that does not read changes nothing.
func (m *manager) reload(ctx context.Context) error {
	configFile := m.config.ConfigFileUsed()
	check := viper.New()
	check.SetConfigFile(configFile)
	if err := check.ReadInConfig(); err != nil {
		return errors.Wrapf(err, "config file %s does not read, nothing restarted", configFile)
	}
	if err := m.services.Stop(ctx); err != nil {
		m.log.Errorf("failed to stop services for the reload: %s", err)
	}
	// changed again since the check: viper keeps what it had, and the services start on that
	if err := m.config.ReadInConfig(); err != nil {
		m.log.Errorf("failed to re-read config file %s, starting on the config already read: %s", configFile, err)
	}
	// unlike the boot, a failed Init starts what it can: the process is
	// serving, the error is logged and in /readyz, and the supervisor starts
	// no service whose Init or dependencies failed, for the monitor to restart
	initErr := m.services.Init(ctx)
	return errors.Wrap(errors.Combine(initErr, m.services.Start(ctx)))
}
