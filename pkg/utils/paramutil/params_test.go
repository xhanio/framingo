package paramutil_test

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/xhanio/framingo/pkg/utils/paramutil"
)

// query is the notation of a URL-style query: pairs joined by "&", opened by a
// "?" that follows something which is not a pair.
var query = paramutil.NewParams("=", "&", paramutil.WithOpener("?"))

// keyword is the notation of a libpq-style connection string, which is nothing
// but pairs joined by spaces.
var keyword = paramutil.NewParams("=", " ")

func TestParams_Parse(t *testing.T) {
	t.Run("splits the prefix from the pairs it opens", func(t *testing.T) {
		prefix, entries := query.Parse("root:pw@tcp(localhost:3306)/mydb?charset=utf8&loc=Local")
		assert.Equal(t, "root:pw@tcp(localhost:3306)/mydb?", prefix)
		assert.Equal(t, []paramutil.Param[string]{
			{Key: "charset", Value: "utf8", Keyed: true},
			{Key: "loc", Value: "Local", Keyed: true},
		}, entries.List())
	})

	t.Run("reads a string that is nothing but pairs", func(t *testing.T) {
		prefix, entries := keyword.Parse("host=localhost port=5432")
		assert.Empty(t, prefix)
		assert.Equal(t, []paramutil.Param[string]{
			{Key: "host", Value: "localhost", Keyed: true},
			{Key: "port", Value: "5432", Keyed: true},
		}, entries.List())
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
		assert.Equal(t, "host=localhost password=p q ", prefix)
		assert.Equal(t, []paramutil.Param[string]{{Key: "dbname", Value: "mydb", Keyed: true}}, entries.List())
	})

	t.Run("keeps the separator that joined the prefix to its section", func(t *testing.T) {
		prefix, entries := query.Parse("a=1&junk&b=2")
		assert.Equal(t, "a=1&junk&", prefix)
		assert.Equal(t, []paramutil.Param[string]{{Key: "b", Value: "2", Keyed: true}}, entries.List())
	})

	t.Run("returns nothing for an empty string", func(t *testing.T) {
		prefix, entries := paramutil.Flags.Parse("")
		assert.Empty(t, prefix)
		assert.Equal(t, 0, entries.Len())
	})

	t.Run("refuses a key that looks like a uri", func(t *testing.T) {
		prefix, entries := keyword.Parse("postgres://user:pw@host/mydb?sslmode=require")
		assert.Equal(t, "postgres://user:pw@host/mydb?sslmode=require", prefix)
		assert.Equal(t, 0, entries.Len())
	})

	t.Run("collapses a key the string repeats", func(t *testing.T) {
		_, entries := query.Parse("db?charset=utf8&charset=latin1")
		assert.Equal(t, []paramutil.Param[string]{{Key: "charset", Value: "latin1", Keyed: true}}, entries.List())
	})
}

func TestParams_ParseTokens(t *testing.T) {
	t.Run("reads each token as a pair", func(t *testing.T) {
		entries := paramutil.Keyword.ParseTokens([]string{"A=1", "B=2"})
		assert.Equal(t, []paramutil.Param[string]{
			{Key: "A", Value: "1", Keyed: true},
			{Key: "B", Value: "2", Keyed: true},
		}, entries.List())
	})

	t.Run("keeps a token that is not a pair, notation raw or not", func(t *testing.T) {
		entries := paramutil.Keyword.ParseTokens([]string{"A=1", "junk", "B=2"})
		assert.Equal(t, []paramutil.Param[string]{
			{Key: "A", Value: "1", Keyed: true},
			{Raw: "junk"},
			{Key: "B", Value: "2", Keyed: true},
		}, entries.List())
	})

	t.Run("collapses a repeated key onto its first position", func(t *testing.T) {
		entries := paramutil.Keyword.ParseTokens([]string{"A=1", "B=2", "A=3"})
		assert.Equal(t, []paramutil.Param[string]{
			{Key: "A", Value: "3", Keyed: true},
			{Key: "B", Value: "2", Keyed: true},
		}, entries.List())
	})

	t.Run("takes the notation's key prefixes", func(t *testing.T) {
		entries := paramutil.Flags.ParseTokens([]string{"mycmd", "--a=b", "-v"})
		assert.Equal(t, []paramutil.Param[string]{
			{Raw: "mycmd"},
			{Key: "--a", Value: "b", Keyed: true},
			{Raw: "-v"},
		}, entries.List())
	})

	t.Run("takes the notation's quotes", func(t *testing.T) {
		entries := paramutil.Keyword.ParseTokens([]string{"password=''"})
		got, ok := entries.Get("password")
		assert.True(t, ok)
		assert.Empty(t, got)
	})

	t.Run("leaves a separator inside a token alone", func(t *testing.T) {
		entries := paramutil.Flags.ParseTokens([]string{"--a=1 2"})
		got, ok := entries.Get("--a")
		assert.True(t, ok)
		assert.Equal(t, "1 2", got)
	})

	t.Run("returns an empty entries for no tokens", func(t *testing.T) {
		assert.Equal(t, 0, paramutil.Keyword.ParseTokens(nil).Len())
	})
}

func TestParams_ParseTokensInto(t *testing.T) {
	t.Run("adds to what the entries already holds", func(t *testing.T) {
		entries := paramutil.NewEntries[string]()
		entries.Set("A", "1")
		paramutil.Keyword.ParseTokensInto(entries, []string{"B=2"})
		assert.Equal(t, []paramutil.Param[string]{
			{Key: "A", Value: "1", Keyed: true},
			{Key: "B", Value: "2", Keyed: true},
		}, entries.List())
	})

	t.Run("overwrites a key the entries already holds, in place", func(t *testing.T) {
		entries := paramutil.NewEntries[string]()
		entries.Set("A", "1")
		entries.Set("B", "2")
		paramutil.Keyword.ParseTokensInto(entries, []string{"A=3"})
		assert.Equal(t, []paramutil.Param[string]{
			{Key: "A", Value: "3", Keyed: true},
			{Key: "B", Value: "2", Keyed: true},
		}, entries.List())
	})

	t.Run("merges sets left to right", func(t *testing.T) {
		entries := paramutil.NewEntries[string]()
		for _, set := range [][]string{{"A=1", "B=2"}, {"B=3", "C=4"}} {
			paramutil.Keyword.ParseTokensInto(entries, set)
		}
		assert.Equal(t, []string{"A=1", "B=3", "C=4"}, paramutil.Keyword.RenderTokens(entries))
	})
}

func TestParams_RenderTokens(t *testing.T) {
	t.Run("writes one token per entry", func(t *testing.T) {
		entries := paramutil.NewEntries[string]()
		entries.Set("A", "1")
		entries.AddRaw("junk")
		entries.Set("B", "2")
		assert.Equal(t, []string{"A=1", "junk", "B=2"}, paramutil.Keyword.RenderTokens(entries))
	})

	t.Run("leaves a separator inside a value unquoted", func(t *testing.T) {
		entries := paramutil.NewEntries[string]()
		entries.Set("--a", "1 2")
		assert.Equal(t, []string{"--a=1 2"}, paramutil.Flags.RenderTokens(entries))
	})

	t.Run("writes an empty value bare where a string would quote it", func(t *testing.T) {
		entries := paramutil.NewEntries[string]()
		entries.Set("password", "")
		assert.Equal(t, []string{"password="}, paramutil.Keyword.RenderTokens(entries))
		assert.Equal(t, "password=''", paramutil.Keyword.Render("", entries))
	})

	t.Run("quotes a value a reader would unquote", func(t *testing.T) {
		entries := paramutil.NewEntries[string]()
		entries.Set("a", "'x'")
		assert.Equal(t, []string{`a="'x'"`}, paramutil.Comma.RenderTokens(entries))
	})

	t.Run("returns nothing for an empty entries", func(t *testing.T) {
		assert.Empty(t, paramutil.Keyword.RenderTokens(paramutil.NewEntries[string]()))
	})

	t.Run("round-trips through ParseTokens", func(t *testing.T) {
		tokens := []string{"mycmd", "--a=1 2", "-v", `--b="'x'"`}
		assert.Equal(t, tokens, paramutil.Flags.RenderTokens(paramutil.Flags.ParseTokens(tokens)))
	})
}

func TestParams_Render(t *testing.T) {
	entries := func(pairs ...string) paramutil.Entries[string] {
		l := paramutil.NewEntries[string]()
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
		assert.Equal(t, "/tmp/test.db", query.Render("/tmp/test.db", paramutil.NewEntries[string]()))
	})

	t.Run("keeps unkeyed entries", func(t *testing.T) {
		l := paramutil.NewEntries[string]()
		l.Set("a", "1")
		l.AddRaw("x")
		assert.Equal(t, "p?a=1&x", query.Render("p", l))
	})
}

func TestParams_AllowRaw(t *testing.T) {
	t.Run("keeps a token that is not a pair as a raw entry", func(t *testing.T) {
		prefix, entries := paramutil.Flags.Parse("mycmd --a=b -v")
		assert.Empty(t, prefix)
		assert.Equal(t, []paramutil.Param[string]{
			{Raw: "mycmd"},
			{Key: "--a", Value: "b", Keyed: true},
			{Raw: "-v"},
		}, entries.List())
	})

	t.Run("renders raw entries where they stood", func(t *testing.T) {
		assert.Equal(t, "mycmd --a=b -v --c=d",
			paramutil.Flags.Apply("mycmd --a=b -v", map[string]string{"c": "d"}))
	})

	t.Run("keeps a bare word inside a query section", func(t *testing.T) {
		f := paramutil.NewParams("=", "&", paramutil.WithOpener("?"), paramutil.AllowRaw())
		assert.Equal(t, "p?a=1&flag&b=3", f.Apply("p?a=1&flag&b=2", map[string]string{"b": "3"}))
	})

	t.Run("leaves text as prefix when raw is not allowed", func(t *testing.T) {
		prefix, entries := paramutil.Keyword.Parse("junk host=h")
		assert.Equal(t, "junk ", prefix)
		assert.Equal(t, []paramutil.Param[string]{{Key: "host", Value: "h", Keyed: true}}, entries.List())
	})
}

func TestParams_Quote(t *testing.T) {
	both := paramutil.NewParams("=", " ", paramutil.WithQuote("'", `"`))

	t.Run("quotes an empty value", func(t *testing.T) {
		assert.Equal(t, "host=h password=''",
			paramutil.Keyword.Apply("host=h", map[string]string{"password": ""}))
	})

	t.Run("quotes a value that holds the separator", func(t *testing.T) {
		assert.Equal(t, "host=h password='p q'",
			paramutil.Keyword.Apply("host=h", map[string]string{"password": "p q"}))
	})

	t.Run("reads a quoted value back", func(t *testing.T) {
		_, entries := paramutil.Keyword.Parse("host=h password='p q' dbname=d")
		assert.Equal(t, []paramutil.Param[string]{
			{Key: "host", Value: "h", Keyed: true},
			{Key: "password", Value: "p q", Keyed: true},
			{Key: "dbname", Value: "d", Keyed: true},
		}, entries.List())
	})

	t.Run("reads an empty quoted value back", func(t *testing.T) {
		_, entries := paramutil.Keyword.Parse("host=h password=''")
		got, ok := entries.Get("password")
		assert.True(t, ok)
		assert.Empty(t, got)
	})

	t.Run("round-trips a quoted value untouched", func(t *testing.T) {
		s := "host=h password='p q' dbname=d"
		assert.Equal(t, s, paramutil.Keyword.Apply(s, nil))
	})

	t.Run("leaves an apostrophe inside a value alone", func(t *testing.T) {
		_, entries := paramutil.Keyword.Parse("host=h user=o'brien")
		got, _ := entries.Get("user")
		assert.Equal(t, "o'brien", got)
	})

	t.Run("takes any of the notation's quotes", func(t *testing.T) {
		_, entries := both.Parse(`a='1 2' b="3 4"`)
		assert.Equal(t, []paramutil.Param[string]{
			{Key: "a", Value: "1 2", Keyed: true},
			{Key: "b", Value: "3 4", Keyed: true},
		}, entries.List())
	})

	t.Run("picks a quote the value does not hold", func(t *testing.T) {
		assert.Equal(t, `a="it's here"`, both.Apply("", map[string]string{"a": "it's here"}))
	})

	t.Run("quotes a value that would read back as quoted", func(t *testing.T) {
		assert.Equal(t, `a="'x'"`, both.Apply("", map[string]string{"a": "'x'"}))
	})

	t.Run("quotes a value that would open a quote", func(t *testing.T) {
		assert.Equal(t, `a="'x",b=2`, paramutil.Comma.Apply("", map[string]string{"a": "'x", "b": "2"}))
	})

	t.Run("wraps with a quote the value does not begin with", func(t *testing.T) {
		assert.Equal(t, `mycmd --a='"x' --b=2`,
			paramutil.Flags.Apply("mycmd", map[string]string{"a": `"x`, "b": "2"}))
	})

	t.Run("leaves a quote after a later equal sign bare", func(t *testing.T) {
		s := paramutil.Comma.Apply("", map[string]string{"a": "b='c", "z": "9"})
		assert.Equal(t, "a=b='c,z=9", s)
		_, entries := paramutil.Comma.Parse(s)
		got, ok := entries.Get("a")
		assert.True(t, ok)
		assert.Equal(t, "b='c", got)
		_, ok = entries.Get("z")
		assert.True(t, ok)
	})

	t.Run("leaves a value bare when the value reads back as itself", func(t *testing.T) {
		assert.Equal(t, "a=1 b=2", paramutil.Keyword.Apply("a=1", map[string]string{"b": "2"}))
	})

	t.Run("writes a bare empty value when the notation has no quotes", func(t *testing.T) {
		assert.Equal(t, "a=&b=1", paramutil.Query.Apply("a=", map[string]string{"b": "1"}))
	})

	t.Run("keeps a quoted value out of the key rules", func(t *testing.T) {
		_, entries := paramutil.Keyword.Parse("host=h path='/var/lib'")
		got, ok := entries.Get("path")
		assert.True(t, ok)
		assert.Equal(t, "/var/lib", got)
	})
}

func TestParams_BadKeyChars(t *testing.T) {
	t.Run("refuses a name holding one of them by default", func(t *testing.T) {
		entries := paramutil.NewParams("=", " ").ParseTokens([]string{"a/b=1"})
		assert.Equal(t, []paramutil.Param[string]{{Raw: "a/b=1"}}, entries.List())
	})

	t.Run("takes any name where a notation names none", func(t *testing.T) {
		f := paramutil.NewParams("=", " ", paramutil.WithBadKeyChars(""))
		entries := f.ParseTokens([]string{"a/b=1", "u@h=2", "c:d=3"})
		assert.Equal(t, []paramutil.Param[string]{
			{Key: "a/b", Value: "1", Keyed: true},
			{Key: "u@h", Value: "2", Keyed: true},
			{Key: "c:d", Value: "3", Keyed: true},
		}, entries.List())
	})

	t.Run("takes the characters a notation does name", func(t *testing.T) {
		f := paramutil.NewParams("=", " ", paramutil.WithBadKeyChars("!"))
		entries := f.ParseTokens([]string{"a/b=1", "c!d=2"})
		assert.Equal(t, []paramutil.Param[string]{
			{Key: "a/b", Value: "1", Keyed: true},
			{Raw: "c!d=2"},
		}, entries.List())
	})

	t.Run("leaves the notations of the package with their default", func(t *testing.T) {
		uri := "postgres://user:pw@host/mydb?sslmode=require"
		prefix, entries := paramutil.Keyword.Parse(uri)
		assert.Equal(t, uri, prefix)
		assert.Equal(t, 0, entries.Len())
	})
}

func TestParams_KeyPrefix(t *testing.T) {
	t.Run("accepts any of the notation's prefixes", func(t *testing.T) {
		_, entries := paramutil.Flags.Parse("-a=1 --b=2")
		assert.Equal(t, []paramutil.Param[string]{
			{Key: "-a", Value: "1", Keyed: true},
			{Key: "--b", Value: "2", Keyed: true},
		}, entries.List())
	})

	t.Run("keeps the prefix a key was written with", func(t *testing.T) {
		assert.Equal(t, "-a=2", paramutil.Flags.Apply("-a=1", map[string]string{"-a": "2"}))
	})

	t.Run("gives a bare name the first prefix of the notation", func(t *testing.T) {
		assert.Equal(t, "--a=1 --b=2", paramutil.Flags.Apply("--a=1", map[string]string{"b": "2"}))
	})

	t.Run("matches a bare name against the key it prefixes to", func(t *testing.T) {
		assert.Equal(t, "--a=2", paramutil.Flags.Apply("--a=1", map[string]string{"a": "2"}))
	})

	t.Run("tells one prefix from another", func(t *testing.T) {
		assert.Equal(t, "-a=1 --a=2", paramutil.Flags.Apply("-a=1 --a=2", nil))
	})

	t.Run("refuses a token that carries no prefix", func(t *testing.T) {
		_, entries := paramutil.Flags.Parse("a=b")
		assert.Equal(t, []paramutil.Param[string]{{Raw: "a=b"}}, entries.List())
	})

	t.Run("refuses a key that is nothing but its prefix", func(t *testing.T) {
		_, entries := paramutil.Flags.Parse("--=b")
		assert.Equal(t, []paramutil.Param[string]{{Raw: "--=b"}}, entries.List())
	})
}

func TestNewParams(t *testing.T) {
	t.Run("does not alias the prefixes or quotes it was given", func(t *testing.T) {
		prefixes := []string{"--"}
		quotes := []string{"'"}
		f := paramutil.NewParams("=", " ",
			paramutil.WithKeyPrefix(prefixes...), paramutil.WithQuote(quotes...))
		prefixes[0] = "++"
		quotes[0] = "~"
		entries := f.ParseTokens([]string{"--a='x'"})
		got, ok := entries.Get("--a")
		assert.True(t, ok)
		assert.Equal(t, "x", got)
	})
}

func TestParams_Notations(t *testing.T) {
	tests := []struct {
		name     string
		notation paramutil.Params
		s        string
		params   map[string]string
		want     string
	}{
		{
			name:     "a=b&c=d",
			notation: paramutil.Query,
			s:        "a=b&c=d",
			params:   map[string]string{"c": "e", "x": "y"},
			want:     "a=b&c=e&x=y",
		},
		{
			name:     "a=b c=d",
			notation: paramutil.Keyword,
			s:        "a=b c=d",
			params:   map[string]string{"c": "e", "x": "y"},
			want:     "a=b c=e x=y",
		},
		{
			name:     "--a=b --c=d",
			notation: paramutil.Flags,
			s:        "--a=b --c=d",
			params:   map[string]string{"c": "e", "x": "y"},
			want:     "--a=b --c=e --x=y",
		},
		{
			name:     "a=b,c=d",
			notation: paramutil.Comma,
			s:        "a=b,c=d",
			params:   map[string]string{"c": "e", "x": "y"},
			want:     "a=b,c=e,x=y",
		},
		{
			name:     "flags that follow a command",
			notation: paramutil.Flags,
			s:        "mycmd --a=b",
			params:   map[string]string{"c": "d"},
			want:     "mycmd --a=b --c=d",
		},
		{
			name:     "a query that follows a url",
			notation: paramutil.Query,
			s:        "https://host/path?a=b",
			params:   map[string]string{"c": "d"},
			want:     "https://host/path?a=b&c=d",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, tt.notation.Apply(tt.s, tt.params))
		})
	}
}

func TestParams_Apply(t *testing.T) {
	tests := []struct {
		name     string
		notation paramutil.Params
		s        string
		params   map[string]string
		want     string
	}{
		{
			name:     "no params leaves the string untouched",
			notation: query,
			s:        "/tmp/test.db",
			want:     "/tmp/test.db",
		},
		{
			name:     "opens the section when the string has none",
			notation: query,
			s:        "/tmp/test.db",
			params:   map[string]string{"_foreign_keys": "on"},
			want:     "/tmp/test.db?_foreign_keys=on",
		},
		{
			name:     "overwrites a pair already in the string, in place",
			notation: query,
			s:        "root:pw@tcp(localhost:3306)/mydb?charset=utf8&parseTime=True&loc=Local",
			params:   map[string]string{"charset": "utf8mb4"},
			want:     "root:pw@tcp(localhost:3306)/mydb?charset=utf8mb4&parseTime=True&loc=Local",
		},
		{
			name:     "appends new pairs in sorted order",
			notation: query,
			s:        "root:pw@tcp(localhost:3306)/mydb?charset=utf8",
			params:   map[string]string{"tls": "true", "timeout": "5s"},
			want:     "root:pw@tcp(localhost:3306)/mydb?charset=utf8&timeout=5s&tls=true",
		},
		{
			name:     "does not repeat an opener the string already ends with",
			notation: query,
			s:        "clickhouse://default:@localhost:9000/default?",
			params:   map[string]string{"secure": "true"},
			want:     "clickhouse://default:@localhost:9000/default?secure=true",
		},
		{
			name:     "overwrites a keyword pair",
			notation: keyword,
			s:        "host=localhost port=5432 sslmode=disable",
			params:   map[string]string{"sslmode": "require"},
			want:     "host=localhost port=5432 sslmode=require",
		},
		{
			name:     "appends to a keyword string without a leading separator",
			notation: keyword,
			s:        "host=localhost port=5432",
			params:   map[string]string{"sslmode": "disable"},
			want:     "host=localhost port=5432 sslmode=disable",
		},
		{
			name:     "keeps an opener that belongs to the prefix",
			notation: query,
			s:        "root:pa?ss@tcp(localhost:3306)/mydb?charset=utf8",
			params:   map[string]string{"charset": "utf8mb4"},
			want:     "root:pa?ss@tcp(localhost:3306)/mydb?charset=utf8mb4",
		},
		{
			name:     "keeps a value that contains the equal sign",
			notation: keyword,
			s:        "host=localhost password=p=q",
			params:   map[string]string{"sslmode": "disable"},
			want:     "host=localhost password=p=q sslmode=disable",
		},
		{
			name:     "honours a notation that is not key=value&key=value",
			notation: paramutil.NewParams(":", ";"),
			s:        "a:1;b:2",
			params:   map[string]string{"b": "3", "c": "4"},
			want:     "a:1;b:3;c:4",
		},
		{
			name:     "is identity with no params around a bare query word",
			notation: paramutil.Query,
			s:        "host/path?debug&b=2",
			want:     "host/path?debug&b=2",
		},
		{
			name:     "merges into the pairs after a bare query word",
			notation: paramutil.Query,
			s:        "host/path?debug&b=2",
			params:   map[string]string{"b": "3"},
			want:     "host/path?debug&b=3",
		},
		{
			name:     "is identity with no params around a doubled separator",
			notation: paramutil.Query,
			s:        "host/path?a=1&&b=2",
			want:     "host/path?a=1&&b=2",
		},
		{
			name:     "does not open a section after a trailing separator",
			notation: paramutil.Query,
			s:        "db?a=1&",
			params:   map[string]string{"a": "2"},
			want:     "db?a=1&a=2",
		},
		{
			name:     "applies to an empty string without a stray separator",
			notation: paramutil.Flags,
			s:        "",
			params:   map[string]string{"a": "1"},
			want:     "--a=1",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, tt.notation.Apply(tt.s, tt.params))
		})
	}
}
