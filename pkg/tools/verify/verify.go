// Package verify gives one verdict on a change: format, build, vet, lint and
// tests, by the standard tools, for each module directory given. The framingo
// command's verify runs it.
package verify

import (
	"context"
	"fmt"
	"io"
	"os"
	"os/exec"
	"strings"

	"github.com/xhanio/errors"
)

const lintInstall = "go install github.com/golangci/golangci-lint/v2/cmd/golangci-lint@latest"

// a step's output is shown only when it fails
type step struct {
	name string
	run  func(ctx context.Context, dir string) (string, error)
}

// Run verifies each module directory, the current one when none is given,
// writing a line per step to w. Every step runs even after one fails, so one
// run shows everything to fix; the error says the change is not verified.
func Run(ctx context.Context, w io.Writer, dirs ...string) error {
	return run(ctx, w, steps(), dirs...)
}

func steps() []step {
	return []step{
		{"gofmt", gofmt},
		{"go build", command("go", "build", "./...")},
		{"go vet", command("go", "vet", "./...")},
		{"golangci-lint", lint},
		{"go test -race", command("go", "test", "-race", "-count=1", "-timeout", "10m", "./...")},
	}
}

func run(ctx context.Context, w io.Writer, steps []step, dirs ...string) error {
	if len(dirs) == 0 {
		dirs = []string{"."}
	}
	var failed []string
	for _, dir := range dirs {
		fmt.Fprintf(w, "== %s\n", dir)
		if info, err := os.Stat(dir); err != nil || !info.IsDir() {
			fmt.Fprintf(w, "  FAIL  no such directory\n")
			failed = append(failed, dir+": no such directory")
			continue
		}
		for _, s := range steps {
			out, err := s.run(ctx, dir)
			if err == nil {
				fmt.Fprintf(w, "  ok    %s\n", s.name)
				continue
			}
			fmt.Fprintf(w, "  FAIL  %s\n", s.name)
			if out = strings.TrimRight(out, "\n"); out == "" {
				out = err.Error()
			}
			for _, line := range strings.Split(out, "\n") {
				fmt.Fprintf(w, "        %s\n", line)
			}
			failed = append(failed, dir+": "+s.name)
		}
	}
	if len(failed) == 0 {
		fmt.Fprintln(w, "verified")
		return nil
	}
	fmt.Fprintln(w, "not verified:")
	for _, f := range failed {
		fmt.Fprintf(w, "  %s\n", f)
	}
	return errors.Newf("not verified: %d step(s) failed", len(failed))
}

func command(name string, args ...string) func(context.Context, string) (string, error) {
	return func(ctx context.Context, dir string) (string, error) {
		cmd := exec.CommandContext(ctx, name, args...)
		cmd.Dir = dir
		out, err := cmd.CombinedOutput()
		return string(out), err
	}
}

// gofmt fails on any file gofmt would change, generated output and the
// skill's templates aside.
func gofmt(ctx context.Context, dir string) (string, error) {
	out, err := command("gofmt", "-l", ".")(ctx, dir)
	if err != nil {
		return out, err
	}
	if files := unformatted(out); len(files) > 0 {
		return strings.Join(files, "\n"), errors.Newf("%d file(s) not gofmt-clean", len(files))
	}
	return "", nil
}

func unformatted(gofmtOutput string) []string {
	var files []string
	for _, f := range strings.Fields(gofmtOutput) {
		if strings.Contains("/"+f, "/_templates/") || strings.Contains("/"+f, "/dist/") {
			continue
		}
		files = append(files, f)
	}
	return files
}

func lint(ctx context.Context, dir string) (string, error) {
	if _, err := exec.LookPath("golangci-lint"); err != nil {
		return "golangci-lint is not installed: " + lintInstall, err
	}
	return command("golangci-lint", "run", "--timeout", "10m", "./...")(ctx, dir)
}
