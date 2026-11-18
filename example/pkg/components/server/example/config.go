package example

import (
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
	// Every key can be overridden from the environment, and the variable is
	// the prefix plus the key with its dots as underscores: db.source.host
	// is FRAMINGO_EXAMPLE_DB_SOURCE_HOST (plain DB_SOURCE_HOST in a binary
	// built without gopro, which injects the product name). Without the
	// replacer viper would look for FRAMINGO_EXAMPLE_DB.SOURCE.HOST, which
	// no shell or manifest sets.
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
	m.config.WatchConfig()
	absPath, err := filepath.Abs(configFile)
	if err != nil {
		return errors.Wrapf(err, "failed to resolve config path %s", configFile)
	}
	infra.ConfigDir = filepath.Dir(absPath)
	return nil
}
