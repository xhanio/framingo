package example

import (
	"context"
	"fmt"
	"io"
	"net/http"
	_ "net/http/pprof"
	"os/signal"
	"path/filepath"
	"syscall"
	"time"

	"github.com/spf13/viper"
	"github.com/xhanio/errors"
	"github.com/xhanio/framingo/pkg/services/api/server"
	"github.com/xhanio/framingo/pkg/services/app"
	"github.com/xhanio/framingo/pkg/services/db"
	"github.com/xhanio/framingo/pkg/types/api"
	"github.com/xhanio/framingo/pkg/types/info"
	"github.com/xhanio/framingo/pkg/utils/certutil"
	"github.com/xhanio/framingo/pkg/utils/envutil"
	"github.com/xhanio/framingo/pkg/utils/log"
	"github.com/xhanio/framingo/pkg/utils/sliceutil"
	"go.uber.org/zap/zapcore"

	// "github.com/xhanio/framingo/pkg/services/common/eventbus"

	"github.com/xhanio/framingo/example/pkg/services/example"
	"github.com/xhanio/framingo/example/pkg/utils/infra"
)

type manager struct {
	config *viper.Viper
	// util services
	log log.Logger
	db  db.Manager

	// system services

	// business services
	example example.Manager

	// api related services
	mws []api.Middleware
	api server.Manager

	// service controller
	services app.Manager
}

func New(config Config) Server {
	conf := viper.New()
	conf.SetConfigFile(config.Path)
	infra.EnvPrefix = envutil.EnvPrefix(info.ProductName)
	conf.SetEnvPrefix(infra.EnvPrefix)
	conf.AutomaticEnv()
	return &manager{
		config: conf,
	}
}

func (m *manager) Init(ctx context.Context) error {
	infra.StartTime = time.Now()
	configFile := m.config.ConfigFileUsed()
	if err := m.config.ReadInConfig(); err != nil {
		return errors.Wrapf(err, "failed to read config file %s", configFile)
	}
	m.config.WatchConfig()
	configPath, err := filepath.Abs(configFile)
	if err != nil {
		return errors.Wrapf(err, "failed to resolve config path %s", configFile)
	}
	infra.ConfigDir = filepath.Dir(configPath)

	// init logger
	m.log = log.New(
		log.WithLevel(m.config.GetInt("log.level")),
		log.WithFileWriter(
			m.config.GetString("log.file"),
			m.config.GetInt("log.rotation.max_size"),
			m.config.GetInt("log.rotation.max_backups"),
			m.config.GetInt("log.rotation.max_age"),
		),
	)
	infra.Debug = (m.log.Level() == zapcore.DebugLevel)

	// init db manager
	m.db = db.New(
		db.WithType(m.config.GetString("db.type")),
		db.WithDataSource(db.Source{
			Host: sliceutil.First(
				m.config.GetString("db.source.host"),
				m.config.GetString("DB_HOST"),
				"127.0.0.1",
			),
			Port: sliceutil.First(
				m.config.GetUint("db.source.port"),
				m.config.GetUint("DB_PORT"),
				5432,
			),
			User: sliceutil.First(
				m.config.GetString("db.source.user"),
				m.config.GetString("DB_USER"),
			),
			Password: sliceutil.First(
				m.config.GetString("db.source.password"),
				m.config.GetString("DB_PASSWORD"),
			),
			DBName: sliceutil.First(
				m.config.GetString("db.source.dbname"),
				m.config.GetString("DB_DBNAME"),
			),
		}),
		db.WithMigration(
			m.config.GetString("db.migration.dir"),
			m.config.GetUint("db.migration.version"),
		),
		db.WithConnection(
			m.config.GetInt("db.connection.max_open"),
			m.config.GetInt("db.connection.max_idle"),
			m.config.GetDuration("db.connection.max_lifetime"),
			m.config.GetDuration("db.connection.exec_timeout"),
		),
		db.WithLogger(m.log),
	)

	// init service manager
	m.services = app.New(m.config,
		app.WithLogger(m.log),
	)

	/* init utility level components */

	/* init system level components */

	/* init business level components */

	m.example = example.New(
		m.db,
		example.WithLogger(m.log),
	)

	/* init api level components and register all routers and grpc services */

	// init api manager
	m.api = server.New(
		server.WithLogger(m.log),
	)

	// iterate over api configurations
	servers := m.config.GetStringMap("api")
	for name := range servers {
		opts := []server.ServerOption{
			server.WithEndpoint(
				m.config.GetString(fmt.Sprintf("api.%s.host", name)),
				m.config.GetUint(fmt.Sprintf("api.%s.port", name)),
				m.config.GetString(fmt.Sprintf("api.%s.prefix", name)),
			),
		}
		// add throttle if configured
		if m.config.IsSet(fmt.Sprintf("api.%s.throttle", name)) {
			opts = append(opts, server.WithThrottle(
				m.config.GetFloat64(fmt.Sprintf("api.%s.throttle.rps", name)),
				m.config.GetInt(fmt.Sprintf("api.%s.throttle.burst_size", name)),
			))
		}
		// add TLS if configured
		if m.config.IsSet(fmt.Sprintf("api.%s.cert", name)) {
			opts = append(opts, server.WithTLS(
				certutil.MustCAFromFile(
					m.config.GetString("ca.cert"),
					m.config.GetString(fmt.Sprintf("api.%s.cert", name)),
					m.config.GetString(fmt.Sprintf("api.%s.key", name)),
				),
				true,
			))
		}
		if err := m.api.Add(name, opts...); err != nil {
			return errors.Wrap(err)
		}
	}

	// init grpc manager
	// m.grpc = grpc.New(
	// 	grpc.WithLogger(m.log),
	// )

	// register basic services
	m.services.Register(
		m.db,
	)

	// register system services
	m.services.Register(
		m.example,
	)

	// register business services

	// perform a topo sort to ensure the dependencies
	if err := m.services.TopoSort(); err != nil {
		return errors.Wrap(err)
	}

	// append api & grpc after topo sort to ensure the latest start
	m.services.Register(
		m.api,
	)

	// register to eventbus
	// if err := m.registerEventHandler(m.services.Services()...); err != nil {
	// 	return errors.Wrap(err)
	// }

	/* pre initialization */

	// init all services
	if err := m.services.Init(ctx); err != nil {
		m.log.Error(err)
	}

	/* post initialization */

	if err := m.initAPI(); err != nil {
		return errors.Wrap(err)
	}

	return nil
}

func (m *manager) Start(ctx context.Context) error {
	// enable pprof
	pport := m.config.GetUint("pprof.port")
	if pport != 0 {
		go func() {
			m.log.Infof("enable pprof on port %d", pport)
			err := http.ListenAndServe(fmt.Sprintf("localhost:%d", pport), nil)
			if err != nil {
				panic(err)
			}
		}()
	}
	if err := m.services.Start(ctx); err != nil {
		return err
	}
	// block until interrupted — SIGUSR1/SIGUSR2 are handled by the app manager
	ctx, stop := signal.NotifyContext(ctx, syscall.SIGINT, syscall.SIGTERM)
	defer stop()
	<-ctx.Done()
	m.log.Info("gracefully shutdown manager")
	return m.services.Stop(true)
}

func (m *manager) Stop(wait bool) error {
	if err := m.services.Stop(wait); err != nil {
		m.log.Error(err)
	}
	return nil
}

func (m *manager) Info(w io.Writer, debug bool) {
	m.services.Info(w, debug)
}
