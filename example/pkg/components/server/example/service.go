package example

import (
	"fmt"

	"github.com/xhanio/errors"
	"go.uber.org/zap/zapcore"
	"gopkg.in/yaml.v3"
	// framingo's own tracing package already owns the name here
	gormtracing "gorm.io/plugin/opentelemetry/tracing"

	tracemw "github.com/xhanio/framingo/pkg/middlewares/trace"
	"github.com/xhanio/framingo/pkg/services/api/server"
	"github.com/xhanio/framingo/pkg/services/db"
	_ "github.com/xhanio/framingo/pkg/services/db/drivers/clickhouse"
	_ "github.com/xhanio/framingo/pkg/services/db/drivers/mysql"
	_ "github.com/xhanio/framingo/pkg/services/db/drivers/postgres"
	_ "github.com/xhanio/framingo/pkg/services/db/drivers/sqlite"
	"github.com/xhanio/framingo/pkg/services/db/plugins/otelgorm"
	"github.com/xhanio/framingo/pkg/services/messagebus"
	"github.com/xhanio/framingo/pkg/services/metrics"
	_ "github.com/xhanio/framingo/pkg/services/metrics/exporters/otlpgrpc"
	_ "github.com/xhanio/framingo/pkg/services/metrics/exporters/otlphttp"
	_ "github.com/xhanio/framingo/pkg/services/metrics/exporters/stdout"
	"github.com/xhanio/framingo/pkg/services/pubsub"
	"github.com/xhanio/framingo/pkg/services/pubsub/driver"
	"github.com/xhanio/framingo/pkg/services/supervisor"
	"github.com/xhanio/framingo/pkg/services/tracing"
	_ "github.com/xhanio/framingo/pkg/services/tracing/exporters/otlpgrpc"
	_ "github.com/xhanio/framingo/pkg/services/tracing/exporters/otlphttp"
	_ "github.com/xhanio/framingo/pkg/services/tracing/exporters/stdout"
	"github.com/xhanio/framingo/pkg/utils/certutil"
	"github.com/xhanio/framingo/pkg/utils/confutil"
	"github.com/xhanio/framingo/pkg/utils/log"
	"github.com/xhanio/framingo/pkg/utils/sliceutil"

	corsmw "github.com/xhanio/framingo/example/pkg/middlewares/cors"
	"github.com/xhanio/framingo/example/pkg/services/example"
	"github.com/xhanio/framingo/example/pkg/services/repository"
	"github.com/xhanio/framingo/example/pkg/services/system/auth"
	"github.com/xhanio/framingo/example/pkg/services/system/certificate"
	"github.com/xhanio/framingo/example/pkg/services/system/organization"
	"github.com/xhanio/framingo/example/pkg/services/system/platform"
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

	// init service manager - the wiring layer owns the viper (file, env
	// prefix, watch); services see it only as the read-side Config the
	// supervisor carries into Init
	m.services = supervisor.New(confutil.New(m.config),
		supervisor.WithLogger(m.log),
	)

	/* init infra level services */

	// tracing.* is read here like every other service's config; Init re-reads
	// the same keys, so a restart picks up changes to any of them
	m.tracing = tracing.New(
		tracing.WithEnabled(m.config.GetBool("tracing.enabled")),
		tracing.WithExporter(
			m.config.GetString("tracing.exporter"),
			m.config.GetStringMap("tracing.config"),
		),
		tracing.WithSampleRatio(m.config.GetFloat64("tracing.sample_ratio")),
		tracing.WithLogger(m.log),
	)

	// metrics.* follows the same rule; the meter's provider is stable, so
	// services create their instruments once at construction
	m.metrics = metrics.New(
		metrics.WithEnabled(m.config.GetBool("metrics.enabled")),
		metrics.WithExporter(
			m.config.GetString("metrics.exporter"),
			m.config.GetStringMap("metrics.config"),
		),
		metrics.WithInterval(m.config.GetDuration("metrics.interval")),
		metrics.WithLogger(m.log),
	)

	// per-statement spans, nested under whatever span the caller's context
	// carries. Off in practice until tracing.enabled is true: the plugin
	// resolves a noop provider then. Bound values are left out of
	// db.query.text - upstream includes them by default, and they are where
	// the credentials and the PII are.
	dbTracing, err := otelgorm.New(m.tracing, gormtracing.WithoutQueryVariables())
	if err != nil {
		return errors.Wrap(err)
	}

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
		db.WithPlugins(dbTracing),
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

	// combine the services business constructors consume
	m.platform = platform.New(m.db, m.metrics, m.pubsub, m.messagebus)

	m.repository = repository.New(
		m.platform,
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
		m.platform,
		m.repository,
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
			// trace and cors must see every request - spans should cover the
			// whole chain, and a preflight OPTIONS matches no route - so both
			// ride the server-level slot, trace first so cors runs inside the
			// span. The api.<name>.middlewares mapping activates each by name
			// ("trace: true", "cors: true"); with no entry an attachment
			// stays dormant, so the health listener serves with neither.
			server.WithMiddlewares(
				tracemw.New(m.tracing),
				corsmw.New(),
			),
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
