package docs_test

import (
	"encoding/json"
	"os"
	"path/filepath"
	"regexp"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/xhanio/framingo/pkg/tools/docs"
)

// the repository this package lives in is the first it checks
const repo = "../../.."

// plans and specs kept for the record, some pointing at files since moved
var skipped = []string{"docs/superpowers", "dist", "example/dist"}

func TestDocLinksReachWhatTheyNameTo(t *testing.T) {
	problems, err := docs.Links(repo, skipped...)
	require.NoError(t, err)
	for _, p := range problems {
		t.Error(p)
	}
}

// A snippet marked "<!-- source: path -->" shows that file's functions and
// types as they are, comments aside.
func TestDocSnippetsMatchTheirSource(t *testing.T) {
	problems, err := docs.Snippets(repo, skipped...)
	require.NoError(t, err)
	for _, p := range problems {
		t.Error(p)
	}
}

// A release bumps every surface at once: the plugin's version in two places,
// the framework version it documents in five.
func TestVersionSurfacesAgree(t *testing.T) {
	find := func(file, pattern string) string {
		t.Helper()
		data, err := os.ReadFile(filepath.Join(repo, file))
		require.NoError(t, err)
		m := regexp.MustCompile(pattern).FindSubmatch(data)
		require.NotNil(t, m, "%s: no match for %s", file, pattern)
		return string(m[1])
	}
	const skill = "plugins/framingo/skills/framingo/SKILL.md"
	const manifest = "plugins/framingo/.claude-plugin/plugin.json"

	var plugin struct {
		Version     string `json:"version"`
		Description string `json:"description"`
	}
	data, err := os.ReadFile(filepath.Join(repo, manifest))
	require.NoError(t, err)
	require.NoError(t, json.Unmarshal(data, &plugin))

	assert.Equal(t, plugin.Version, find(skill, `(?m)^  version: "([^"]+)"`), "the skill's metadata.version mirrors plugin.json")

	framework := find("example/project.yaml", `(?m)^version: (v\S+)`)
	for surface, got := range map[string]string{
		manifest + " description":      find(manifest, `pinned to framingo (v[0-9.]+\d)`),
		skill + " compatibility":       find(skill, `Documents github\.com/xhanio/framingo (v[0-9.]+\d)`),
		skill + " metadata.framingo":   find(skill, `(?m)^  framingo: (v\S+)`),
		"example/go.mod's requirement": find("example/go.mod", `(?m)^\tgithub\.com/xhanio/framingo (v\S+)`),
	} {
		assert.Equal(t, framework, got, "%s names the version example/project.yaml does", surface)
	}
}
