package example

import (
	"fmt"

	"github.com/xhanio/errors"
	"github.com/xhanio/framingo/pkg/services/api/server"
	"github.com/xhanio/framingo/pkg/services/db"
	_ "github.com/xhanio/framingo/pkg/services/db/drivers/clickhouse"
	_ "github.com/xhanio/framingo/pkg/services/db/drivers/mysql"
	_ "github.com/xhanio/framingo/pkg/services/db/drivers/postgres"
	_ "github.com/xhanio/framingo/pkg/services/db/drivers/sqlite"
	"github.com/xhanio/framingo/pkg/services/messagebus"
	"github.com/xhanio/framingo/pkg/services/pubsub"
	"github.com/xhanio/framingo/pkg/services/pubsub/driver"
	"github.com/xhanio/framingo/pkg/services/supervisor"
	"github.com/xhanio/framingo/pkg/utils/certutil"
	"github.com/xhanio/framingo/pkg/utils/log"
	"github.com/xhanio/framingo/pkg/utils/sliceutil"
	"go.uber.org/zap/zapcore"
	"gopkg.in/yaml.v3"

	corsmw "github.com/xhanio/framingo/example/pkg/middlewares/cors"
	"github.com/xhanio/framingo/example/pkg/services/example"
	"github.com/xhanio/framingo/example/pkg/services/repository"
	"github.com/xhanio/framingo/example/pkg/services/system/auth"
	"github.com/xhanio/framingo/example/pkg/services/system/certificate"
	"github.com/xhanio/framingo/example/pkg/services/system/organization"
	"github.com/xhanio/framingo/example/pkg/services/system/role"
	"github.com/xhanio/framingo/example/pkg/services/system/user"
	"github.com/xhanio/framingo/example/pkg/utils/infra"
)

func (m *manager) initServices() error {
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

	// init service manager
	m.services = supervisor.New(m.config,
		supervisor.WithLogger(m.log),
	)

	/* init infra level services */

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
			m.config.GetDuration("db.connection.max_idle_time"),
			m.config.GetDuration("db.connection.exec_timeout"),
		),
		db.WithLogger(m.log),
	)

	m.pubsub = pubsub.New(
		driver.NewMemory(m.log),
		pubsub.WithLogger(m.log),
	)

	m.messagebus = messagebus.New(
		m.pubsub,
		messagebus.WithLogger(m.log),
	)

	m.repository = repository.New(
		m.db,
		repository.WithLogger(m.log),
	)

	/* init system level services */

	m.user = user.New(
		m.repository,
		user.WithLogger(m.log),
	)

	m.role = role.New(
		m.repository,
		role.WithLogger(m.log),
	)

	m.organization = organization.New(
		m.repository,
		organization.WithLogger(m.log),
	)

	m.certificate = certificate.New(
		m.repository,
		certificate.WithLogger(m.log),
	)

	m.auth = auth.New(
		m.user,
		nil, // LDAPAuthN is optional
		nil, // APITokenAuthN is optional
		auth.WithLogger(m.log),
	)

	/* init business level components */

	m.example = example.New(
		m.repository,
		m.messagebus,
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
			// cors must see every request - a preflight OPTIONS matches no
			// route - so it rides the server-level slot. The
			// api.<name>.middlewares mapping activates it under "cors" and
			// carries its policy; with no entry it stays dormant, so the
			// health listener serves without it.
			server.WithMiddlewares(corsmw.New()),
		}
		// Per-server middleware configs: a plain mapping of middleware name to
		// its default config, unordered - order only matters in router.yaml,
		// where attachment lives. Each block reaches its middleware wherever
		// nothing more specific is configured, the server's own built-ins
		// (cors, recover, logger, info, error) included.
		if mws := m.config.GetStringMap(fmt.Sprintf("api.%s.middlewares", name)); len(mws) > 0 {
			raw, err := yaml.Marshal(mws)
			if err != nil {
				return errors.Wrap(err)
			}
			opts = append(opts, server.WithMiddlewareConfigs(raw))
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

	return nil
}
