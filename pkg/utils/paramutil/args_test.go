package paramutil_test

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/xhanio/framingo/pkg/utils/paramutil"
)

func TestArgs_Parse(t *testing.T) {
	t.Run("takes the token after a flag as its value", func(t *testing.T) {
		entries := paramutil.Argv.Parse([]string{"-b", "2"})
		assert.Equal(t, []paramutil.Param[paramutil.Arg]{
			{Key: "-b", Value: paramutil.Arg{Values: []string{"2"}}, Keyed: true},
		}, entries.List())
	})

	t.Run("takes every token up to the next flag", func(t *testing.T) {
		entries := paramutil.Argv.Parse([]string{"-b", "2", "3", "-c"})
		assert.Equal(t, []paramutil.Param[paramutil.Arg]{
			{Key: "-b", Value: paramutil.Arg{Values: []string{"2", "3"}}, Keyed: true},
			{Key: "-c", Keyed: true},
		}, entries.List())
	})

	t.Run("closes a flag written with an equal sign", func(t *testing.T) {
		entries := paramutil.Argv.Parse([]string{"--out=json", "file.txt"})
		assert.Equal(t, []paramutil.Param[paramutil.Arg]{
			{Key: "--out", Value: paramutil.Arg{Inline: true, Values: []string{"json"}}, Keyed: true},
			{Raw: "file.txt"},
		}, entries.List())
	})

	t.Run("keeps a positional that opens the line", func(t *testing.T) {
		entries := paramutil.Argv.Parse([]string{"build", "-x"})
		assert.Equal(t, []paramutil.Param[paramutil.Arg]{
			{Raw: "build"},
			{Key: "-x", Keyed: true},
		}, entries.List())
	})

	t.Run("reads a bare dash as a positional", func(t *testing.T) {
		entries := paramutil.Argv.Parse([]string{"cat", "-"})
		assert.Equal(t, []paramutil.Param[paramutil.Arg]{{Raw: "cat"}, {Raw: "-"}}, entries.List())
	})

	t.Run("reads a bare double dash as a flag, not a terminator", func(t *testing.T) {
		entries := paramutil.Argv.Parse([]string{"rm", "--", "somefile"})
		assert.Equal(t, []paramutil.Param[paramutil.Arg]{
			{Raw: "rm"},
			{Key: "--", Value: paramutil.Arg{Values: []string{"somefile"}}, Keyed: true},
		}, entries.List())
	})

	t.Run("replaces a value list as a unit", func(t *testing.T) {
		entries := paramutil.Argv.Parse([]string{"-b", "2", "3", "-b", "9"})
		assert.Equal(t, []paramutil.Param[paramutil.Arg]{
			{Key: "-b", Value: paramutil.Arg{Values: []string{"9"}}, Keyed: true},
		}, entries.List())
	})
}

func TestNewArgs(t *testing.T) {
	t.Run("does not alias the prefixes it was given", func(t *testing.T) {
		prefixes := []string{"--"}
		a := paramutil.NewArgs("=", prefixes)
		prefixes[0] = "+"
		assert.Equal(t, []paramutil.Param[paramutil.Arg]{{Key: "--a", Keyed: true}},
			a.Parse([]string{"--a"}).List())
	})
}

func TestArgs_ParseInto(t *testing.T) {
	t.Run("does not leave a flag open across calls", func(t *testing.T) {
		entries := paramutil.NewEntries[paramutil.Arg]()
		paramutil.Argv.ParseInto(entries, []string{"-b"})
		paramutil.Argv.ParseInto(entries, []string{"2"})
		assert.Equal(t, []paramutil.Param[paramutil.Arg]{
			{Key: "-b", Keyed: true},
			{Raw: "2"},
		}, entries.List())
	})

	t.Run("merges sets left to right", func(t *testing.T) {
		entries := paramutil.NewEntries[paramutil.Arg]()
		for _, set := range [][]string{{"-a", "-b", "2"}, {"-b", "3"}} {
			paramutil.Argv.ParseInto(entries, set)
		}
		assert.Equal(t, []string{"-a", "-b", "3"}, paramutil.Argv.Render(entries))
	})
}

func TestArgs_Render(t *testing.T) {
	t.Run("writes a flag and its values as separate tokens", func(t *testing.T) {
		entries := paramutil.NewEntries[paramutil.Arg]()
		entries.Set("-b", paramutil.Arg{Values: []string{"2", "3"}})
		assert.Equal(t, []string{"-b", "2", "3"}, paramutil.Argv.Render(entries))
	})

	t.Run("writes an inline flag as one token", func(t *testing.T) {
		entries := paramutil.NewEntries[paramutil.Arg]()
		entries.Set("--out", paramutil.Arg{Inline: true, Values: []string{"json"}})
		assert.Equal(t, []string{"--out=json"}, paramutil.Argv.Render(entries))
	})

	t.Run("writes an inline flag given no value", func(t *testing.T) {
		entries := paramutil.NewEntries[paramutil.Arg]()
		entries.Set("--out", paramutil.Arg{Inline: true})
		assert.Equal(t, []string{"--out="}, paramutil.Argv.Render(entries))
	})

	t.Run("returns an empty line rather than nothing", func(t *testing.T) {
		got := paramutil.Argv.Render(paramutil.NewEntries[paramutil.Arg]())
		assert.NotNil(t, got)
		assert.Empty(t, got)
	})

	t.Run("round-trips a line", func(t *testing.T) {
		tokens := []string{"build", "--out=json", "-b", "2", "3", "-", "-v"}
		assert.Equal(t, tokens, paramutil.Argv.Render(paramutil.Argv.Parse(tokens)))
	})
}
