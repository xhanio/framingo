package verify

import (
	"bytes"
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/xhanio/errors"
)

func fake(name, out string, err error, ran *[]string) step {
	return step{name, func(_ context.Context, dir string) (string, error) {
		*ran = append(*ran, dir+":"+name)
		return out, err
	}}
}

func TestRunVerifiesEveryStepOfEveryDir(t *testing.T) {
	var ran []string
	var w bytes.Buffer
	a, b := t.TempDir(), t.TempDir()
	err := run(context.Background(), &w, []step{fake("one", "", nil, &ran), fake("two", "", nil, &ran)}, a, b)
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
	err := run(context.Background(), &w, []step{
		fake("one", "a.go:1: bad\na.go:2: worse\n", broken, &ran),
		fake("two", "", nil, &ran),
		fake("three", "", broken, &ran),
	}, dir)
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
	err := run(context.Background(), &w, []step{fake("one", "", nil, &ran)}, dir+"/missing", dir)
	require.Error(t, err)
	assert.Equal(t, []string{dir + ":one"}, ran)
	assert.Contains(t, w.String(), "missing: no such directory")
}

func TestUnformattedLeavesOutTemplatesAndGeneratedOutput(t *testing.T) {
	out := "pkg/a.go\nplugins/framingo/skills/framingo/_templates/handler.go\ndist/local/x.go\nexample/dist/y.go\ncmd/b.go\n"
	assert.Equal(t, []string{"pkg/a.go", "cmd/b.go"}, unformatted(out))
	assert.Empty(t, unformatted(""))
}
