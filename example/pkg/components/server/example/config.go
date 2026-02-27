package example

import (
	"path/filepath"
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
	conf.AutomaticEnv()
	return conf
}

func (m *manager) initConfig() error {
	infra.StartTime = time.Now()
	configFile := m.config.ConfigFileUsed()
	if err := m.config.ReadInConfig(); err != nil {
		return errors.Wrapf(err, "failed to read config file %s", configFile)
	}
	m.config.WatchConfig()
	absPath, err := filepath.Abs(configFile)
	if err != nil {
		return errors.Wrapf(err, "failed to resolve config path %s", configFile)
	}
	infra.ConfigDir = filepath.Dir(absPath)
	return nil
}
