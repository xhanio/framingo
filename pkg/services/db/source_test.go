package db_test

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/xhanio/framingo/pkg/services/db"
	_ "github.com/xhanio/framingo/pkg/services/db/drivers/clickhouse"
	_ "github.com/xhanio/framingo/pkg/services/db/drivers/mysql"
	_ "github.com/xhanio/framingo/pkg/services/db/drivers/postgres"
	_ "github.com/xhanio/framingo/pkg/services/db/drivers/sqlite"
)

func TestSource_DSN(t *testing.T) {
	tests := []struct {
		name    string
		source  db.Source
		dbtype  string
		want    string
		wantErr bool
	}{
		{
			name: "Postgres default",
			source: db.Source{
				Host:     "localhost",
				Port:     5432,
				User:     "user",
				Password: "password",
				DBName:   "mydb",
			},
			dbtype: db.Postgres,
			want:   "host=localhost port=5432 user=user password=password dbname=mydb sslmode=disable",
		},
		{
			name: "Postgres with SSL",
			source: db.Source{
				Host:     "localhost",
				Port:     5432,
				User:     "user",
				Password: "password",
				DBName:   "mydb",
				Secure:   true,
			},
			dbtype: db.Postgres,
			want:   "host=localhost port=5432 user=user password=password dbname=mydb sslmode=require",
		},
		{
			name: "Postgres with an empty password",
			source: db.Source{
				Host:   "localhost",
				Port:   5432,
				User:   "user",
				DBName: "mydb",
			},
			dbtype: db.Postgres,
			want:   "host=localhost port=5432 user=user password='' dbname=mydb sslmode=disable",
		},
		{
			name:   "SQLite memory",
			source: db.Source{},
			dbtype: db.SQLite,
			want:   ":memory:",
		},
		{
			name: "SQLite file",
			source: db.Source{
				DBName: "/tmp/test.db",
			},
			dbtype: db.SQLite,
			want:   "/tmp/test.db",
		},
		{
			name: "Postgres with Params override",
			source: db.Source{
				Host:     "localhost",
				Port:     5432,
				User:     "user",
				Password: "password",
				DBName:   "mydb",
				Secure:   true,
				Params:   map[string]string{"sslmode": "verify-full"},
			},
			dbtype: db.Postgres,
			want:   "host=localhost port=5432 user=user password=password dbname=mydb sslmode=verify-full",
		},
		{
			name: "MySQL with SSL and Params",
			source: db.Source{
				Host:     "localhost",
				Port:     3306,
				User:     "root",
				Password: "password",
				DBName:   "mydb",
				Secure:   true,
				Params:   map[string]string{"timeout": "5s"},
			},
			dbtype: db.MySQL,
			want:   "root:password@tcp(localhost:3306)/mydb?charset=utf8&parseTime=True&loc=Local&timeout=5s&tls=true",
		},
		{
			name: "MySQL with Params override",
			source: db.Source{
				Host:     "localhost",
				Port:     3306,
				User:     "root",
				Password: "password",
				DBName:   "mydb",
				Secure:   true,
				Params:   map[string]string{"tls": "skip-verify"},
			},
			dbtype: db.MySQL,
			want:   "root:password@tcp(localhost:3306)/mydb?charset=utf8&parseTime=True&loc=Local&tls=skip-verify",
		},
		{
			name: "MySQL Params override a built-in default",
			source: db.Source{
				Host:     "localhost",
				Port:     3306,
				User:     "root",
				Password: "password",
				DBName:   "mydb",
				Params:   map[string]string{"charset": "utf8mb4", "loc": "UTC"},
			},
			dbtype: db.MySQL,
			want:   "root:password@tcp(localhost:3306)/mydb?charset=utf8mb4&parseTime=True&loc=UTC",
		},
		{
			name: "ClickHouse with SSL and Params",
			source: db.Source{
				Host:     "localhost",
				Port:     9000,
				User:     "default",
				Password: "",
				DBName:   "default",
				Secure:   true,
				Params:   map[string]string{"debug": "true"},
			},
			dbtype: db.Clickhouse,
			want:   "clickhouse://default:@localhost:9000/default?debug=true&secure=true",
		},
		{
			name: "ClickHouse with Params override",
			source: db.Source{
				Host:     "localhost",
				Port:     9000,
				User:     "default",
				Password: "",
				DBName:   "default",
				Secure:   true,
				Params:   map[string]string{"secure": "false"},
			},
			dbtype: db.Clickhouse,
			want:   "clickhouse://default:@localhost:9000/default?secure=false",
		},
		{
			name: "SQLite with Params",
			source: db.Source{
				DBName: "/tmp/test.db",
				Params: map[string]string{"_foreign_keys": "on"},
			},
			dbtype: db.SQLite,
			want:   "/tmp/test.db?_foreign_keys=on",
		},
		{
			name:    "unregistered driver",
			source:  db.Source{},
			dbtype:  "duckdb",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := tt.source.DSN(tt.dbtype)
			if tt.wantErr {
				assert.Error(t, err)
				return
			}
			assert.NoError(t, err)
			assert.Equal(t, tt.want, got)
		})
	}
}
