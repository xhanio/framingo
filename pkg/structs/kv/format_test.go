package kv_test

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/xhanio/framingo/pkg/structs/kv"
)

// query is the notation of a URL-style query: pairs joined by "&", opened by a
// "?" that follows something which is not a pair.
var query = kv.NewFormat("=", "&", kv.WithOpener("?"))

// keyword is the notation of a libpq-style connection string, which is nothing
// but pairs joined by spaces.
var keyword = kv.NewFormat("=", " ")

func TestFormat_Parse(t *testing.T) {
	t.Run("splits the prefix from the pairs it opens", func(t *testing.T) {
		prefix, entries := query.Parse("root:pw@tcp(localhost:3306)/mydb?charset=utf8&loc=Local")
		assert.Equal(t, "root:pw@tcp(localhost:3306)/mydb?", prefix)
		assert.Equal(t, []kv.Entry[string]{
			{Key: "charset", Value: "utf8", Keyed: true},
			{Key: "loc", Value: "Local", Keyed: true},
		}, entries.Entries())
	})

	t.Run("reads a string that is nothing but pairs", func(t *testing.T) {
		prefix, entries := keyword.Parse("host=localhost port=5432")
		assert.Empty(t, prefix)
		assert.Equal(t, []kv.Entry[string]{
			{Key: "host", Value: "localhost", Keyed: true},
			{Key: "port", Value: "5432", Keyed: true},
		}, entries.Entries())
	})

	t.Run("keeps everything as prefix when no section is open", func(t *testing.T) {
		prefix, entries := query.Parse("/tmp/test.db")
		assert.Equal(t, "/tmp/test.db", prefix)
		assert.Equal(t, 0, entries.Len())
	})

	t.Run("keeps a tail that is not a pair as prefix", func(t *testing.T) {
		prefix, entries := query.Parse("/tmp/we?ird.db")
		assert.Equal(t, "/tmp/we?ird.db", prefix)
		assert.Equal(t, 0, entries.Len())
	})

	t.Run("stops at a token that is not a pair", func(t *testing.T) {
		prefix, entries := keyword.Parse("host=localhost password=p q dbname=mydb")
		assert.Equal(t, "host=localhost password=p q", prefix)
		assert.Equal(t, []kv.Entry[string]{{Key: "dbname", Value: "mydb", Keyed: true}}, entries.Entries())
	})

	t.Run("refuses a key that looks like a uri", func(t *testing.T) {
		prefix, entries := keyword.Parse("postgres://user:pw@host/mydb?sslmode=require")
		assert.Equal(t, "postgres://user:pw@host/mydb?sslmode=require", prefix)
		assert.Equal(t, 0, entries.Len())
	})

	t.Run("collapses a key the string repeats", func(t *testing.T) {
		_, entries := query.Parse("db?charset=utf8&charset=latin1")
		assert.Equal(t, []kv.Entry[string]{{Key: "charset", Value: "latin1", Keyed: true}}, entries.Entries())
	})
}

func TestFormat_Render(t *testing.T) {
	entries := func(pairs ...string) kv.List[string] {
		l := kv.New[string]()
		for i := 0; i < len(pairs); i += 2 {
			l.Set(pairs[i], pairs[i+1])
		}
		return l
	}

	t.Run("opens the section after a prefix", func(t *testing.T) {
		assert.Equal(t, "/tmp/test.db?a=1&b=2", query.Render("/tmp/test.db", entries("a", "1", "b", "2")))
	})

	t.Run("does not repeat an opener the prefix already ends with", func(t *testing.T) {
		assert.Equal(t, "db?a=1", query.Render("db?", entries("a", "1")))
	})

	t.Run("writes bare pairs when there is no prefix", func(t *testing.T) {
		assert.Equal(t, "host=h port=1", keyword.Render("", entries("host", "h", "port", "1")))
	})

	t.Run("returns the prefix alone when there are no pairs", func(t *testing.T) {
		assert.Equal(t, "/tmp/test.db", query.Render("/tmp/test.db", kv.New[string]()))
	})

	t.Run("keeps unkeyed entries", func(t *testing.T) {
		l := kv.New[string]()
		l.Set("a", "1")
		l.AddRaw("x")
		assert.Equal(t, "p?a=1&x", query.Render("p", l))
	})
}

func TestFormat_Notations(t *testing.T) {
	tests := []struct {
		name   string
		format kv.Format
		s      string
		params map[string]string
		want   string
	}{
		{
			name:   "a=b&c=d",
			format: kv.Query,
			s:      "a=b&c=d",
			params: map[string]string{"c": "e", "x": "y"},
			want:   "a=b&c=e&x=y",
		},
		{
			name:   "a=b c=d",
			format: kv.Keyword,
			s:      "a=b c=d",
			params: map[string]string{"c": "e", "x": "y"},
			want:   "a=b c=e x=y",
		},
		{
			name:   "--a=b --c=d",
			format: kv.Flags,
			s:      "--a=b --c=d",
			params: map[string]string{"c": "e", "x": "y"},
			want:   "--a=b --c=e --x=y",
		},
		{
			name:   "a=b,c=d",
			format: kv.Comma,
			s:      "a=b,c=d",
			params: map[string]string{"c": "e", "x": "y"},
			want:   "a=b,c=e,x=y",
		},
		{
			name:   "flags that follow a command",
			format: kv.Flags,
			s:      "mycmd --a=b",
			params: map[string]string{"c": "d"},
			want:   "mycmd --a=b --c=d",
		},
		{
			name:   "a query that follows a url",
			format: kv.Query,
			s:      "https://host/path?a=b",
			params: map[string]string{"c": "d"},
			want:   "https://host/path?a=b&c=d",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, tt.format.Apply(tt.s, tt.params))
		})
	}
}

func TestFormat_Apply(t *testing.T) {
	tests := []struct {
		name   string
		format kv.Format
		s      string
		params map[string]string
		want   string
	}{
		{
			name:   "no params leaves the string untouched",
			format: query,
			s:      "/tmp/test.db",
			want:   "/tmp/test.db",
		},
		{
			name:   "opens the section when the string has none",
			format: query,
			s:      "/tmp/test.db",
			params: map[string]string{"_foreign_keys": "on"},
			want:   "/tmp/test.db?_foreign_keys=on",
		},
		{
			name:   "overwrites a pair already in the string, in place",
			format: query,
			s:      "root:pw@tcp(localhost:3306)/mydb?charset=utf8&parseTime=True&loc=Local",
			params: map[string]string{"charset": "utf8mb4"},
			want:   "root:pw@tcp(localhost:3306)/mydb?charset=utf8mb4&parseTime=True&loc=Local",
		},
		{
			name:   "appends new pairs in sorted order",
			format: query,
			s:      "root:pw@tcp(localhost:3306)/mydb?charset=utf8",
			params: map[string]string{"tls": "true", "timeout": "5s"},
			want:   "root:pw@tcp(localhost:3306)/mydb?charset=utf8&timeout=5s&tls=true",
		},
		{
			name:   "does not repeat an opener the string already ends with",
			format: query,
			s:      "clickhouse://default:@localhost:9000/default?",
			params: map[string]string{"secure": "true"},
			want:   "clickhouse://default:@localhost:9000/default?secure=true",
		},
		{
			name:   "overwrites a keyword pair",
			format: keyword,
			s:      "host=localhost port=5432 sslmode=disable",
			params: map[string]string{"sslmode": "require"},
			want:   "host=localhost port=5432 sslmode=require",
		},
		{
			name:   "appends to a keyword string without a leading separator",
			format: keyword,
			s:      "host=localhost port=5432",
			params: map[string]string{"sslmode": "disable"},
			want:   "host=localhost port=5432 sslmode=disable",
		},
		{
			name:   "keeps an opener that belongs to the prefix",
			format: query,
			s:      "root:pa?ss@tcp(localhost:3306)/mydb?charset=utf8",
			params: map[string]string{"charset": "utf8mb4"},
			want:   "root:pa?ss@tcp(localhost:3306)/mydb?charset=utf8mb4",
		},
		{
			name:   "keeps a value that contains the equal sign",
			format: keyword,
			s:      "host=localhost password=p=q",
			params: map[string]string{"sslmode": "disable"},
			want:   "host=localhost password=p=q sslmode=disable",
		},
		{
			name:   "honours a notation that is not key=value&key=value",
			format: kv.NewFormat(":", ";"),
			s:      "a:1;b:2",
			params: map[string]string{"b": "3", "c": "4"},
			want:   "a:1;b:3;c:4",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, tt.format.Apply(tt.s, tt.params))
		})
	}
}
