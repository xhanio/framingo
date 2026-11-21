package docs_test

import (
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

const skill = repo + "/plugins/framingo/skills/framingo"

// a scaffold's own code, which the templates leave to the app
var stubs = map[string]string{
	"pkg/types/entity/order.go": `package entity

type Order struct {
	ID       string ` + "`json:\"id\"`" + `
	Item     string ` + "`json:\"item\"`" + `
	Quantity int    ` + "`json:\"quantity\"`" + `
}
`,
	"pkg/types/api/order.go": `package api

type CreateOrderRequest struct {
	Item     string ` + "`json:\"item\" validate:\"required\"`" + `
	Quantity int    ` + "`json:\"quantity\" validate:\"gt=0\"`" + `
}
`,
	"pkg/types/repo/order.go": `package repo

import (
	"context"

	"github.com/yourorg/myapp/pkg/types/entity"
)

type Order interface {
	GetOrder(ctx context.Context, id string) (*entity.Order, error)
	CreateOrder(ctx context.Context, order *entity.Order) error
}
`,
}

var copyLine = regexp.MustCompile(`(?m)^cp <skill>/_templates/(\S+)\s+(\S+)$`)

// An app laid out as SKILL.md's Route B copies the templates passes framingo
// verify: whatever the templates show is what the app's checks then hold.
func TestTemplatesPassVerify(t *testing.T) {
	if testing.Short() {
		t.Skip("builds and lints a scaffold")
	}
	if _, err := exec.LookPath("golangci-lint"); err != nil {
		t.Skip("golangci-lint is not installed")
	}
	root, err := filepath.Abs(repo)
	require.NoError(t, err)
	dir := t.TempDir()

	instructions, err := os.ReadFile(filepath.Join(skill, "SKILL.md"))
	require.NoError(t, err)
	copied := map[string]bool{}
	for _, m := range copyLine.FindAllStringSubmatch(string(instructions), -1) {
		for _, name := range expand(m[1]) {
			dst := m[2]
			if strings.HasSuffix(dst, "/") {
				dst += name
			}
			write(t, filepath.Join(dir, dst), read(t, filepath.Join(skill, "_templates", name)))
			copied[name] = true
		}
	}
	templates, err := os.ReadDir(filepath.Join(skill, "_templates"))
	require.NoError(t, err)
	for _, e := range templates {
		if !copied[e.Name()] {
			t.Errorf("SKILL.md copies no _templates/%s", e.Name())
		}
	}
	for path, content := range stubs {
		write(t, filepath.Join(dir, path), content)
	}
	write(t, filepath.Join(dir, "go.mod"), "module github.com/yourorg/myapp\n\ngo 1.25.0\n\n"+
		"require github.com/xhanio/framingo v0.0.0\n\nreplace github.com/xhanio/framingo => "+root+"\n")
	write(t, filepath.Join(dir, "go.sum"), read(t, filepath.Join(root, "go.sum")))

	// from the module cache alone: the framework's own build has filled it
	run(t, dir, "go", "mod", "tidy")
	bin := filepath.Join(t.TempDir(), "framingo")
	run(t, root, "go", "build", "-o", bin, ".")
	run(t, dir, bin, "verify", ".")
}

// expand is a shell brace list's names: {a,b} is a and b.
func expand(s string) []string {
	if strings.HasPrefix(s, "{") && strings.HasSuffix(s, "}") {
		return strings.Split(s[1:len(s)-1], ",")
	}
	return []string{s}
}

func run(t *testing.T, dir, name string, args ...string) {
	t.Helper()
	cmd := exec.Command(name, args...)
	cmd.Dir = dir
	cmd.Env = append(os.Environ(), "GOWORK=off", "GOFLAGS=-mod=mod", "GOPROXY=off")
	out, err := cmd.CombinedOutput()
	require.NoError(t, err, "%s %s:\n%s", name, strings.Join(args, " "), out)
}

func read(t *testing.T, path string) string {
	t.Helper()
	b, err := os.ReadFile(path)
	require.NoError(t, err)
	return string(b)
}

func write(t *testing.T, path, content string) {
	t.Helper()
	require.NoError(t, os.MkdirAll(filepath.Dir(path), 0o755))
	require.NoError(t, os.WriteFile(path, []byte(content), 0o644))
}
