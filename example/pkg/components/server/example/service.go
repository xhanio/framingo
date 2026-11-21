package example

import (
	"fmt"
	"time"

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

func (m *manager) newLogger() log.Logger {
	return log.New(
		log.WithLevel(m.config.GetInt("log.level")),
		log.WithFileWriter(
			m.config.GetString("log.file"),
			m.config.GetInt("log.rotation.max_size"),
			m.config.GetInt("log.rotation.max_backups"),
			m.config.GetInt("log.rotation.max_age"),
		),
	)
}

func (m *manager) newDatabase(opts ...db.Option) db.Manager {
	base := []db.Option{
		db.WithType(m.config.GetString("db.type")),
		// env beats config.yaml; fallbacks apply only when neither sets a value
		db.WithDataSource(db.Source{
			Host:     sliceutil.First(m.config.GetString("db.source.host"), "127.0.0.1"),
			Port:     sliceutil.First(m.config.GetUint("db.source.port"), 5432),
			User:     m.config.GetString("db.source.user"),
			Password: m.config.GetString("db.source.password"),
			DBName:   m.config.GetString("db.source.dbname"),
		}),
		// empty dir: the daemon never touches the schema (the migrate Job does)
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
	}
	return db.New(append(base, opts...)...)
}

func (m *manager) initServices() error {
	m.log = m.newLogger()
	infra.Debug = (m.log.Level() == zapcore.DebugLevel)

	// services see the viper only as the read-side Config the supervisor carries into Init
	m.services = supervisor.New(confutil.New(m.config),
		supervisor.WithLogger(m.log),
		// at boot, what does not come up is waited for, as long as it takes:
		// the wait doubles from 1s to one try in 5m, each blocked try logged
		// with its error; a reload's Init makes one pass, a broken file failing it
		supervisor.WithInitPolicy(-1, time.Second, 5*time.Minute),
		// a Start that takes seconds is stuck: failing it here records why,
		// within the liveness probe's budget for Init and every Start
		supervisor.WithStartPolicy(10*time.Second),
		// within the platform's stop timeout, so shutdown ends here, not in a SIGKILL
		supervisor.WithStopPolicy(20*time.Second),
		// probes feed /healthz and /readyz: without a sweep they would report
		// only what Init and Start left behind
		supervisor.WithMonitorPolicy(
			m.config.GetDuration("supervisor.monitor.interval"),
			m.config.GetInt("supervisor.monitor.max_retries"),
			m.config.GetDuration("supervisor.monitor.restart_delay"),
		),
	)

	// Init re-reads these keys, so a restart picks up changes
	m.tracing = tracing.New(
		tracing.WithEnabled(m.config.GetBool("tracing.enabled")),
		tracing.WithExporter(
			m.config.GetString("tracing.exporter"),
			m.config.GetStringMap("tracing.config"),
		),
		tracing.WithSampleRatio(m.config.GetFloat64("tracing.sample_ratio")),
		tracing.WithLogger(m.log),
	)

	// the meter's provider is stable, so a service creates its instruments once, at its first Init
	m.metrics = metrics.New(
		metrics.WithEnabled(m.config.GetBool("metrics.enabled")),
		metrics.WithExporter(
			m.config.GetString("metrics.exporter"),
			m.config.GetStringMap("metrics.config"),
		),
		metrics.WithInterval(m.config.GetDuration("metrics.interval")),
		metrics.WithLogger(m.log),
	)

	// bound values are left out of db.query.text: upstream includes them, and they carry credentials and PII
	dbTracing, err := otelgorm.New(m.tracing, gormtracing.WithoutQueryVariables())
	if err != nil {
		return errors.Wrap(err)
	}

	m.db = m.newDatabase(db.WithPlugins(dbTracing))

	m.pubsub = pubsub.New(
		driver.NewMemory(m.log),
		pubsub.WithLogger(m.log),
	)

	m.messagebus = messagebus.New(
		m.pubsub,
		messagebus.WithLogger(m.log),
	)

	m.platform = platform.New(m.db, m.metrics, m.pubsub, m.messagebus)

	// one instance implements every repo contract; each service sees it through its own Repository interface
	m.repository = repository.New(
		m.platform,
		repository.WithLogger(m.log),
	)

	// user publishes the account changes auth ends sessions on
	m.user = user.New(
		m.platform,
		m.repository,
		user.WithLogger(m.log),
	)

	m.role = role.New(
		m.platform,
		m.repository,
		role.WithLogger(m.log),
	)

	m.organization = organization.New(
		m.platform,
		m.repository,
		organization.WithLogger(m.log),
	)

	m.certificate = certificate.New(
		m.platform,
		m.repository,
		certificate.WithLogger(m.log),
	)

	m.auth = auth.New(
		m.platform,
		m.user,
		nil, // LDAPAuthN is optional
		nil, // APITokenAuthN is optional
		auth.WithLogger(m.log),
	)

	m.example = example.New(
		m.platform,
		m.repository,
		example.WithLogger(m.log),
	)

	m.api = server.New(
		server.WithLogger(m.log),
	)

	servers := m.config.GetStringMap("api")
	for name := range servers {
		opts := []server.ServerOption{
			server.WithEndpoint(
				m.config.GetString(fmt.Sprintf("api.%s.host", name)),
				m.config.GetUint(fmt.Sprintf("api.%s.port", name)),
				m.config.GetString(fmt.Sprintf("api.%s.prefix", name)),
			),
			// server-level so they see every request (a preflight matches no route),
			// trace first so cors runs inside the span; dormant unless
			// api.<name>.middlewares enables them by name
			server.WithMiddlewares(
				tracemw.New(m.tracing),
				corsmw.New(),
			),
			// series are keyed by listener, so health probes never distort the public one's
			server.WithMeter(m.metrics),
		}
		// per-server middleware defaults, unordered; order lives in router.yaml
		if mws := m.config.GetStringMap(fmt.Sprintf("api.%s.middlewares", name)); len(mws) > 0 {
			raw, err := yaml.Marshal(mws)
			if err != nil {
				return errors.Wrap(err)
			}
			opts = append(opts, server.WithMiddlewareConfigs(raw))
		}
		if m.config.IsSet(fmt.Sprintf("api.%s.cert", name)) {
			opts = append(opts, server.WithTLS(certutil.MustCertFromFile(
				m.config.GetString(fmt.Sprintf("api.%s.cert", name)),
				"",
				m.config.GetString(fmt.Sprintf("api.%s.key", name)),
			)))
		}
		if err := m.api.Add(name, opts...); err != nil {
			return errors.Wrap(err)
		}
	}

	// m.grpc = grpc.New(
	// 	grpc.WithLogger(m.log),
	// )

	return nil
}
