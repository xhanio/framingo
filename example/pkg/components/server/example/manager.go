package example

import (
	"context"
	"fmt"
	"io"
	"net/http"
	_ "net/http/pprof"
	"os"
	"os/signal"
	"path/filepath"
	"runtime"
	"syscall"
	"time"

	"github.com/spf13/viper"
	"github.com/xhanio/errors"
	"github.com/xhanio/framingo/pkg/services/api/server"
	"github.com/xhanio/framingo/pkg/services/controller"
	"github.com/xhanio/framingo/pkg/services/db"
	"github.com/xhanio/framingo/pkg/types/api"
	"github.com/xhanio/framingo/pkg/types/info"
	"github.com/xhanio/framingo/pkg/utils/certutil"
	"github.com/xhanio/framingo/pkg/utils/log"
	"github.com/xhanio/framingo/pkg/utils/sliceutil"
	"go.uber.org/zap/zapcore"

	// "github.com/xhanio/framingo/pkg/services/common/eventbus"

	"github.com/xhanio/framingo/example/services/example"
	"github.com/xhanio/framingo/example/utils/infra"
)

type manager struct {
	config Config
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
	services controller.Manager
}

func New(config Config) Server {
	return &manager{
		config: config,
	}
}

func (m *manager) Init() error {
	infra.StartTime = time.Now()

	confPath, err := filepath.Abs(m.config.Path)
	if err != nil {
		return errors.Wrap(err)
	}

	infra.EnvPrefix = info.EnvPrefix(info.ProductName)

	viper.SetConfigFile(confPath)
	viper.AutomaticEnv()
	viper.SetEnvPrefix(infra.EnvPrefix)
	if err := viper.ReadInConfig(); err != nil {
		return errors.Wrapf(err, "failed to read config file %s", confPath)
	}
	infra.ConfigDir = filepath.Dir(confPath)

	// init logger
	m.log = log.New(
		log.WithLevel(viper.GetInt("log.level")),
		log.WithFileWriter(
			viper.GetString("log.file"),
			viper.GetInt("log.rotation.max_size"),
			viper.GetInt("log.rotation.max_backups"),
			viper.GetInt("log.rotation.max_age"),
		),
	)
	infra.Debug = (m.log.Level() == zapcore.DebugLevel)

	// init db manager
	m.db = db.New(
		db.WithType(viper.GetString("db.type")),
		db.WithDataSource(db.Source{
			Host: sliceutil.First(
				viper.GetString("db.source.host"),
				viper.GetString("DB_HOST"),
				"127.0.0.1",
			),
			Port: sliceutil.First(
				viper.GetUint("db.source.port"),
				viper.GetUint("DB_PORT"),
				5432,
			),
			User: sliceutil.First(
				viper.GetString("db.source.user"),
				viper.GetString("DB_USER"),
			),
			Password: sliceutil.First(
				viper.GetString("db.source.password"),
				viper.GetString("DB_PASSWORD"),
			),
			DBName: sliceutil.First(
				viper.GetString("db.source.dbname"),
				viper.GetString("DB_DBNAME"),
			),
		}),
		db.WithMigration(
			viper.GetString("db.migration.dir"),
			viper.GetUint("db.migration.version"),
		),
		db.WithConnection(
			viper.GetInt("db.connection.max_open"),
			viper.GetInt("db.connection.max_idle"),
			viper.GetDuration("db.connection.max_lifetime"),
			viper.GetDuration("db.connection.exec_timeout"),
		),
		db.WithLogger(m.log),
	)

	// init service manager
	m.services = controller.New(controller.WithLogger(m.log))

	/* init utility level components */

	/* init system level components */

	/* init business level components */

	// m.example = example.New(
	// 	example.WithLogger(m.log),
	// )

	/* init api level components and register all routers and grpc services */

	// init api manager
	m.api = server.New(
		server.WithLogger(m.log),
	)

	// iterate over api configurations
	servers := viper.GetStringMap("api")
	for name := range servers {
		opts := []server.ServerOption{
			server.WithEndpoint(
				viper.GetString(fmt.Sprintf("api.%s.host", name)),
				viper.GetUint(fmt.Sprintf("api.%s.port", name)),
				viper.GetString(fmt.Sprintf("api.%s.prefix", name)),
			),
		}
		// add throttle if configured
		if viper.IsSet(fmt.Sprintf("api.%s.throttle", name)) {
			opts = append(opts, server.WithThrottle(
				viper.GetFloat64(fmt.Sprintf("api.%s.throttle.rps", name)),
				viper.GetInt(fmt.Sprintf("api.%s.throttle.burst_size", name)),
			))
		}
		// add TLS if configured
		if viper.IsSet(fmt.Sprintf("api.%s.cert", name)) {
			opts = append(opts, server.WithTLS(
				certutil.MustCAFromFile(
					viper.GetString("ca.cert"),
					viper.GetString(fmt.Sprintf("api.%s.cert", name)),
					viper.GetString(fmt.Sprintf("api.%s.key", name)),
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
	if err := m.services.Init(); err != nil {
		m.log.Error(err)
	}

	/* post initialization */

	if err := m.initAPI(); err != nil {
		return errors.Wrap(err)
	}

	return nil
}

func (m *manager) Start(ctx context.Context) error {
	go func() {
		defer func() {
			if r := recover(); r != nil {
				if e, ok := r.(error); ok {
					m.log.Errorf("recover from panic: %s", e.Error())
				} else {
					m.log.Error(r)
				}
			}
		}()
		if err := m.services.Start(ctx); err != nil {
			m.log.Error(err)
		}
	}()
	// enable pprof
	pport := viper.GetUint("pprof.port")
	if pport != 0 {
		go func() {
			m.log.Infof("enable pprof on port %d", pport)
			err := http.ListenAndServe(fmt.Sprintf("localhost:%d", pport), nil)
			if err != nil {
				panic(err)
			}
		}()
	}
	signalCh := make(chan os.Signal, 1)
	signal.Notify(signalCh, syscall.SIGINT, syscall.SIGUSR1, syscall.SIGUSR2)
	for sig := range signalCh {
		switch sig {
		case syscall.SIGUSR1:
			m.services.Info(os.Stdout, true)
		case syscall.SIGUSR2:
			buf := make([]byte, 1<<20)
			n := runtime.Stack(buf, true)
			fmt.Printf("========== stack trace ==========\n\n%s\n=================================\n", buf[:n])
		case os.Interrupt:
			m.log.Infof("gracefully shutdown manager")
			if err := m.services.Stop(true); err != nil {
				m.log.Error(err)
			}
			return nil
		default:
			m.log.Warnf("unknown signal %s", sig.String())
		}
	}
	return nil
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
