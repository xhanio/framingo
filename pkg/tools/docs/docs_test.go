package docs

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func tree(t *testing.T, files map[string]string) string {
	t.Helper()
	root := t.TempDir()
	for name, content := range files {
		path := filepath.Join(root, name)
		require.NoError(t, os.MkdirAll(filepath.Dir(path), 0o755))
		require.NoError(t, os.WriteFile(path, []byte(content), 0o644))
	}
	return root
}

func TestLinks(t *testing.T) {
	root := tree(t, map[string]string{
		"README.md": "# Top\n\n" +
			"[ok](docs/guide.md) [anchor](docs/guide.md#the-daemon-contract) [self](#top) [dir](docs/) [web](https://example.com/x)\n" +
			"[gone](docs/missing.md)\n" +
			"[left out](dist/generated.md)\n" +
			"[bad anchor](docs/guide.md#no-such-heading)\n" +
			"`[in code](nowhere.md)`\n" +
			"```\n[in a fence](nowhere.md)\n```\n",
		"docs/guide.md":           "# Guide\n\n### The `Daemon` contract\n\n[up](../README.md#top) [code](../main.go)\n\n```md\n# not a heading\n```\n",
		"main.go":                 "package main\n",
		"dist/generated.md":       "[broken](nowhere.md)\n",
		".hidden/notes.md":        "[broken](nowhere.md)\n",
		"pkg/testdata/fixture.md": "[broken](nowhere.md)\n",
	})
	problems, err := Links(root, "dist")
	require.NoError(t, err)
	assert.Equal(t, []string{
		"README.md:4: link to docs/missing.md: no such file",
		"README.md:5: link to dist/generated.md: dist is left out of the docs",
		"README.md:6: link to docs/guide.md#no-such-heading: no heading #no-such-heading in docs/guide.md",
	}, problems)
}

const sourceFile = `package svc

import "context"

type manager struct {
	name string // its name
	loop int
}

// Start starts.
func (m *manager) Start(ctx context.Context) error {
	// a comment the docs leave out
	if m.loop > 0 {
		return nil
	}

	m.loop = 1
	return nil
}

func helper() int { return 1 }
`

func TestSnippets(t *testing.T) {
	same := "<!-- source: svc/svc.go -->\n```go\n" +
		"type manager struct {\n\tname string\n\tloop int // counts\n}\n\n" +
		"func (m *manager) Start(ctx context.Context) error {\n\tif m.loop > 0 {\n\t\treturn nil\n\t}\n\tm.loop = 1\n\treturn nil\n}\n```\n"
	stale := "<!-- source: svc/svc.go -->\n```go\n" +
		"func (m *manager) Start(ctx context.Context) error {\n\tm.loop = 2\n\treturn nil\n}\n\n" +
		"func (m *manager) Stop() error { return nil }\n```\n"
	root := tree(t, map[string]string{
		"svc/svc.go":  sourceFile,
		"same.md":     "# Same\n\n" + same,
		"stale.md":    "# Stale\n\n" + stale,
		"unmarked.md": "```go\nfunc (m *manager) Start(ctx context.Context) error { return nil }\n```\n",
		"broken.md":   "<!-- source: svc/svc.go -->\n```go\nfunc (m *manager) Start( {\n```\n\n<!-- source: svc/gone.go -->\n```go\nfunc helper() int { return 1 }\n```\n\n<!-- source: svc/svc.go -->\nno fence here\n",
	})
	problems, err := Snippets(root)
	require.NoError(t, err)
	require.Len(t, problems, 5, "%v", problems)
	assert.Contains(t, problems[0], "broken.md:1: the snippet does not parse")
	assert.Contains(t, problems[1], "broken.md:6: cannot read the source")
	assert.Equal(t, "broken.md:11: a source marker with no ```go fence on the next line", problems[2])
	assert.Equal(t, "stale.md:3: func (*manager) Start differs from svc/svc.go", problems[3])
	assert.Equal(t, "stale.md:3: func (*manager) Stop is not in svc/svc.go", problems[4])
}
