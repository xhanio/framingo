package vet

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestAsTool(t *testing.T) {
	for _, args := range [][]string{{"-V=full"}, {"-flags"}, {"-unsafeptr=false", "/tmp/go-build1/b042/vet.cfg"}} {
		assert.True(t, AsTool(args), "%v is go vet's call", args)
	}
	for _, args := range [][]string{nil, {"verify", ".", "example"}, {"vet", "./..."}, {"version"}, {"--help"}} {
		assert.False(t, AsTool(args), "%v is a person's", args)
	}
}

func TestArgs(t *testing.T) {
	assert.Equal(t, []string{"vet", "-vettool=/bin/framingo", "./..."}, Args("/bin/framingo"))
	assert.Equal(t, []string{"vet", "-vettool=/bin/framingo", "./pkg/a", "./pkg/b"}, Args("/bin/framingo", "./pkg/a", "./pkg/b"))
}

func TestAnalyzersAreNamedForTheirRule(t *testing.T) {
	seen := map[string]bool{}
	for _, a := range Analyzers() {
		assert.NotEmpty(t, a.Doc, a.Name)
		assert.False(t, seen[a.Name], "analyzer %s listed twice", a.Name)
		seen[a.Name] = true
	}
}
