# Database Service

A comprehensive database service package for managing database connections, migrations, and operations across multiple database systems using GORM.

## Overview

The `db` service provides a unified interface for database management with support for multiple database types including PostgreSQL, MySQL, SQLite, and ClickHouse. It handles connection pooling, migrations, cleanup operations, and context-aware query execution.

## Supported Databases

- **SQLite** - Lightweight file-based database
- **MySQL** - Popular open-source relational database
- **PostgreSQL** - Advanced open-source relational database
- **ClickHouse** - Fast columnar OLAP database

## Key Features

- Multi-database support with consistent API
- Database migration management using [golang-migrate](https://github.com/golang-migrate/migrate)
- Connection pool configuration
- Context-aware queries with timeout support
- Database cleanup and reload operations
- Integrated logging with configurable levels
- Transaction support through context wrapping

## Usage

### Creating a New Database Manager

```go
import "github.com/xhanio/framingo/pkg/services/db"

manager := db.New(
    db.WithType(db.Postgres),
    db.WithDataSource(db.Source{
        Host:     "localhost",
        Port:     5432,
        User:     "user",
        Password: "password",
        DBName:   "mydb",
    }),
    db.WithConnection(10, 5, time.Minute*5, time.Second*30),
    db.WithMigration("/path/to/migrations", 0),
    db.WithLogger(logger),
)

// Initialize the connection
if err := manager.Init(); err != nil {
    log.Fatal(err)
}
```

### Configuration Options

#### `WithType(dbtype string)`
Sets the database type. Available options:
- `db.SQLite`
- `db.MySQL`
- `db.Postgres`
- `db.Clickhouse`

#### `WithDataSource(source Source)`
Configures the database connection parameters:
```go
type Source struct {
    Host     string
    Port     uint
    User     string
    Password string
    DBName   string
}
```

#### `WithConnection(maxOpen, maxIdle int, maxLifetime, execTimeout time.Duration)`
Configures connection pool settings:
- `maxOpen`: Maximum number of open connections (default: 10)
- `maxIdle`: Maximum number of idle connections (default: 5)
- `maxLifetime`: Maximum connection lifetime (default: 5 minutes)
- `execTimeout`: Default query execution timeout (default: 30 seconds)

#### `WithMigration(sqlDir string, version uint)`
Configures database migrations:
- `sqlDir`: Directory containing migration files
- `version`: Target migration version (0 for latest)

#### `WithLogger(logger log.Logger)`
Sets a custom logger instance.

#### `WithName(name string)`
Sets a custom name for the service.

### Accessing Database Instances

```go
// Get GORM instance
ormDB := manager.ORM()

// Get standard database/sql instance
sqlDB := manager.DB()

// Get context-aware GORM instance
db := manager.FromContext(ctx)

// Get GORM instance with timeout
db, cancel := manager.FromContextTimeout(ctx, time.Second*5)
defer cancel()
```

### Context Integration

The service supports transaction propagation through context:

```go
// Wrap a transaction in context
tx := manager.ORM().Begin()
ctx = db.WrapContext(ctx, tx)

// Use the transaction from context
db := manager.FromContext(ctx)
db.Create(&user)

tx.Commit()
```

### Database Operations

#### Cleanup

Remove all data from the database:

```go
// Truncate all tables (preserves schema)
err := manager.Cleanup(false)

// Drop and recreate schema (removes all tables)
err := manager.Cleanup(true)
```

Database-specific cleanup behavior:
- **PostgreSQL**: Truncates tables with `RESTART IDENTITY CASCADE` or drops/recreates public schema
- **MySQL**: Truncates tables with foreign key checks disabled or drops/recreates database
- **SQLite**: Deletes all rows or drops all tables
- **ClickHouse**: Truncates tables or drops/recreates database

#### Reload

Drop the schema and re-run migrations:

```go
err := manager.Reload()
```

This operation:
1. Drops the entire schema using `Cleanup(true)`
2. Re-runs migrations to the configured version

### Migrations

The service uses [golang-migrate](https://github.com/golang-migrate/migrate) for database migrations.

Migration files should be placed in the configured directory with the naming convention:
```
{version}_{description}.up.sql
{version}_{description}.down.sql
```

Example:
```
001_create_users_table.up.sql
001_create_users_table.down.sql
002_add_email_to_users.up.sql
002_add_email_to_users.down.sql
```

Migrations are automatically run during initialization if configured:
```go
db.WithMigration("/path/to/migrations", 0) // 0 = latest version
db.WithMigration("/path/to/migrations", 3) // migrate to version 3
```

## Interface

The `Manager` interface provides:

```go
type Manager interface {
    common.Service
    common.Initializable
    common.Debuggable

    ORM() *gorm.DB
    DB() *sql.DB
    FromContext(ctx context.Context) *gorm.DB
    FromContextTimeout(ctx context.Context, timeout time.Duration) (*gorm.DB, context.CancelFunc)
    Cleanup(schema bool) error
    Reload() error
}
```

## Debug Information

Get detailed information about the database connection:

```go
// Print connection statistics
manager.Info(os.Stdout, true)
```

This displays:
- Data source configuration
- Connection pool settings
- Migration configuration
- Database connection statistics

## Examples

### PostgreSQL with Migrations

```go
manager := db.New(
    db.WithType(db.Postgres),
    db.WithDataSource(db.Source{
        Host:     "localhost",
        Port:     5432,
        User:     "postgres",
        Password: "secret",
        DBName:   "myapp",
    }),
    db.WithMigration("./migrations", 0),
)
manager.Init()
```

### MySQL with Custom Connection Pool

```go
manager := db.New(
    db.WithType(db.MySQL),
    db.WithDataSource(db.Source{
        Host:     "localhost",
        Port:     3306,
        User:     "root",
        Password: "password",
        DBName:   "myapp",
    }),
    db.WithConnection(20, 10, time.Minute*10, time.Second*60),
)
manager.Init()
```

### SQLite for Testing

```go
manager := db.New(
    db.WithType(db.SQLite),
    db.WithDataSource(db.Source{}), // Uses :memory: database
)
manager.Init()
defer manager.Cleanup(true)
```

### Context-Aware Queries

```go
ctx := context.Background()

// Query with default timeout
db := manager.FromContext(ctx)
db.First(&user, id)

// Query with custom timeout
db, cancel := manager.FromContextTimeout(ctx, time.Second*5)
defer cancel()
db.Find(&users)
```

## Dependencies

- [GORM](https://gorm.io/) - ORM library
- [golang-migrate](https://github.com/golang-migrate/migrate) - Database migrations
- [zapgorm2](https://github.com/moul/zapgorm2) - Zap logger integration for GORM
- [github.com/xhanio/errors](https://github.com/xhanio/errors) - Error handling
- [github.com/xhanio/framingo/pkg/utils/log](../../utils/log) - Logging utilities

## Related Packages

- [pkg/types/common](../../types/common) - Common service interfaces
- [pkg/utils/log](../../utils/log) - Logging utilities
- [pkg/utils/reflectutil](../../utils/reflectutil) - Reflection utilities
- [pkg/utils/printutil](../../utils/printutil) - Printing utilities
