package verify

import (
	"bytes"
	"context"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/xhanio/errors"
)

// fake runs no command: each step's result is scripted by its name.
func fake(results map[string]error, outputs map[string]string, ran *[]string) runner {
	return func(_ context.Context, dir string, s step) (string, error) {
		*ran = append(*ran, dir+":"+s.name)
		return outputs[s.name], results[s.name]
	}
}

func named(names ...string) []step {
	var steps []step
	for _, n := range names {
		steps = append(steps, step{name: n})
	}
	return steps
}

func TestRunVerifiesEveryStepOfEveryDir(t *testing.T) {
	var ran []string
	var w bytes.Buffer
	a, b := t.TempDir(), t.TempDir()
	err := run(context.Background(), &w, named("one", "two"), fake(nil, nil, &ran), a, b)
	require.NoError(t, err)
	assert.Equal(t, []string{a + ":one", a + ":two", b + ":one", b + ":two"}, ran)
	assert.Equal(t, "== "+a+"\n  ok    one\n  ok    two\n== "+b+"\n  ok    one\n  ok    two\nverified\n", w.String())
}

// One run shows everything to fix: a failed step stops nothing after it.
func TestRunGoesOnPastAFailure(t *testing.T) {
	var ran []string
	var w bytes.Buffer
	dir := t.TempDir()
	broken := errors.Newf("exit status 1")
	err := run(context.Background(), &w, named("one", "two", "three"),
		fake(map[string]error{"one": broken, "three": broken}, map[string]string{"one": "a.go:1: bad\na.go:2: worse\n"}, &ran), dir)
	require.Error(t, err)
	assert.Len(t, ran, 3, "every step ran")
	assert.Equal(t, "== "+dir+"\n"+
		"  FAIL  one\n        a.go:1: bad\n        a.go:2: worse\n"+
		"  ok    two\n"+
		"  FAIL  three\n        exit status 1\n"+
		"not verified:\n  "+dir+": one\n  "+dir+": three\n", w.String())
}

func TestRunFailsAMissingDirAndGoesOn(t *testing.T) {
	var ran []string
	var w bytes.Buffer
	dir := t.TempDir()
	err := run(context.Background(), &w, named("one"), fake(nil, nil, &ran), dir+"/missing", dir)
	require.Error(t, err)
	assert.Equal(t, []string{dir + ":one"}, ran)
	assert.Contains(t, w.String(), "missing: no such directory")
}

// execute is the one place a command runs: its output comes back whole, its
// check can fail a zero exit, and a missing binary says how to get it.
func TestExecute(t *testing.T) {
	ctx, dir := context.Background(), t.TempDir()

	out, err := execute(ctx, dir, step{bin: "go", args: []string{"env", "GOOS"}})
	require.NoError(t, err)
	assert.NotEmpty(t, strings.TrimSpace(out))

	out, err = execute(ctx, dir, step{bin: "go", args: []string{"vet", "./nosuchpkg"}})
	require.Error(t, err)
	assert.NotEmpty(t, out, "what the command printed, stderr included")

	_, err = execute(ctx, dir, step{bin: "go", args: []string{"env", "GOOS"}, check: func(string) error { return errors.Newf("checked") }})
	assert.ErrorContains(t, err, "checked")

	out, err = execute(ctx, dir, step{bin: "no-such-binary-anywhere", install: "go install example.com/it@latest"})
	require.Error(t, err)
	assert.Equal(t, "no-such-binary-anywhere is not installed: go install example.com/it@latest", out)
}

func TestStepsVetWithThisBinary(t *testing.T) {
	for _, s := range steps("/bin/framingo") {
		if s.name == "framingo vet" {
			assert.Equal(t, "go", s.bin)
			assert.Equal(t, []string{"vet", "-vettool=/bin/framingo", "./..."}, s.args)
			return
		}
	}
	t.Fatal("no framingo vet step")
}

func TestFormatted(t *testing.T) {
	assert.NoError(t, formatted(""))
	assert.NoError(t, formatted("plugins/framingo/skills/framingo/_templates/handler.go\n"))
	assert.ErrorContains(t, formatted("pkg/a.go\n"), "pkg/a.go")
}

func TestUnformattedLeavesOutTemplatesAndGeneratedOutput(t *testing.T) {
	out := "pkg/a.go\nplugins/framingo/skills/framingo/_templates/handler.go\ndist/local/x.go\nexample/dist/y.go\ncmd/b.go\n"
	assert.Equal(t, []string{"pkg/a.go", "cmd/b.go"}, unformatted(out))
	assert.Empty(t, unformatted(""))
}
