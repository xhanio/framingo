package db

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestSource_DSN(t *testing.T) {
	tests := []struct {
		name    string
		source  Source
		dbtype  string
		want    string
		wantErr bool
	}{
		{
			name: "Postgres default",
			source: Source{
				Host:     "localhost",
				Port:     5432,
				User:     "user",
				Password: "password",
				DBName:   "mydb",
			},
			dbtype: Postgres,
			want:   "host=localhost port=5432 user=user password=password dbname=mydb sslmode=disable",
		},
		{
			name: "Postgres with SSL",
			source: Source{
				Host:     "localhost",
				Port:     5432,
				User:     "user",
				Password: "password",
				DBName:   "mydb",
				Secure:   true,
			},
			dbtype: Postgres,
			want:   "host=localhost port=5432 user=user password=password dbname=mydb sslmode=require",
		},
		{
			name:   "SQLite memory",
			source: Source{},
			dbtype: SQLite,
			want:   ":memory:",
		},
		{
			name: "SQLite file",
			source: Source{
				DBName: "/tmp/test.db",
			},
			dbtype: SQLite,
			want:   "/tmp/test.db",
		},
		{
			name: "Postgres with Params override",
			source: Source{
				Host:     "localhost",
				Port:     5432,
				User:     "user",
				Password: "password",
				DBName:   "mydb",
				Secure:   true,
				Params:   map[string]string{"sslmode": "verify-full"},
			},
			dbtype: Postgres,
			want:   "host=localhost port=5432 user=user password=password dbname=mydb sslmode=verify-full",
		},
		{
			name: "MySQL with SSL and Params",
			source: Source{
				Host:     "localhost",
				Port:     3306,
				User:     "root",
				Password: "password",
				DBName:   "mydb",
				Secure:   true,
				Params:   map[string]string{"timeout": "5s"},
			},
			dbtype: MySQL,
			want:   "root:password@tcp(localhost:3306)/mydb?charset=utf8&parseTime=True&loc=Local&timeout=5s&tls=true",
		},
		{
			name: "MySQL with Params override",
			source: Source{
				Host:     "localhost",
				Port:     3306,
				User:     "root",
				Password: "password",
				DBName:   "mydb",
				Secure:   true,
				Params:   map[string]string{"tls": "skip-verify"},
			},
			dbtype: MySQL,
			want:   "root:password@tcp(localhost:3306)/mydb?charset=utf8&parseTime=True&loc=Local&tls=skip-verify",
		},
		{
			name: "ClickHouse with SSL and Params",
			source: Source{
				Host:     "localhost",
				Port:     9000,
				User:     "default",
				Password: "",
				DBName:   "default",
				Secure:   true,
				Params:   map[string]string{"debug": "true"},
			},
			dbtype: Clickhouse,
			want:   "clickhouse://default:@localhost:9000/default?&debug=true&secure=true",
		},
		{
			name: "ClickHouse with Params override",
			source: Source{
				Host:     "localhost",
				Port:     9000,
				User:     "default",
				Password: "",
				DBName:   "default",
				Secure:   true,
				Params:   map[string]string{"secure": "false"},
			},
			dbtype: Clickhouse,
			want:   "clickhouse://default:@localhost:9000/default?&secure=false",
		},
		{
			name: "SQLite with Params",
			source: Source{
				DBName: "/tmp/test.db",
				Params: map[string]string{"_foreign_keys": "on"},
			},
			dbtype: SQLite,
			want:   "/tmp/test.db?_foreign_keys=on",
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
			// For Postgres with multiple params, the order might vary.
			// If we have multiple params in the map, we should check containment instead of equality.
			// But for the cases above with single param in map, it should be deterministic.
			// Wait, "Postgres with Params override" has 2 params. This will be flaky.
			// I should fix the test case to have only 1 param or handle it.
			// Let's fix the test case in the struct above to have only 1 param for stability.
			if tt.name == "Postgres with Params override" {
				// Special handling or just simplify the test case?
				// Let's simplify the test case in the replacement content to use 1 param.
			}
			assert.Equal(t, tt.want, got)
		})
	}
}
