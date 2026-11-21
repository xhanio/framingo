// Package verify gives one verdict on a change: format, build, vet, the
// framework's own analyzers, lint and tests, by the standard tools, for each
// module directory given. The framingo command's verify runs it.
package verify

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"os"
	"os/exec"
	"strings"

	"github.com/xhanio/errors"

	"github.com/xhanio/framingo/pkg/tools/vet"
	"github.com/xhanio/framingo/pkg/utils/cmdutil"
)

// step is one command run in a module directory; its output is shown only
// when it fails.
type step struct {
	name string
	bin  string
	args []string
	// check fails the step on output its exit status does not speak for; nil: the status decides
	check func(output string) error
	// install is how to get bin, said when it is not found
	install string
}

// runner runs a step in dir and returns its output.
type runner func(ctx context.Context, dir string, s step) (string, error)

// Run verifies each module directory, the current one when none is given,
// writing a line per step to w. Every step runs even after one fails, so one
// run shows everything to fix; the error says the change is not verified.
func Run(ctx context.Context, w io.Writer, dirs ...string) error {
	self, err := os.Executable()
	if err != nil {
		return errors.Wrapf(err, "failed to locate the running binary")
	}
	return run(ctx, w, steps(self), execute, dirs...)
}

// steps are what verify runs, in order; self is the binary go vet takes as its tool.
func steps(self string) []step {
	return []step{
		{name: "gofmt", bin: "gofmt", args: []string{"-l", "."}, check: formatted},
		{name: "go build", bin: "go", args: []string{"build", "./..."}},
		{name: "go vet", bin: "go", args: []string{"vet", "./..."}},
		{name: "framingo vet", bin: "go", args: vet.Args(self)},
		{
			name: "golangci-lint", bin: "golangci-lint", args: []string{"run", "--timeout", "10m", "./..."},
			install: "go install github.com/golangci/golangci-lint/v2/cmd/golangci-lint@latest",
		},
		{name: "go test -race", bin: "go", args: []string{"test", "-race", "-count=1", "-timeout", "10m", "./..."}},
	}
}

func run(ctx context.Context, w io.Writer, steps []step, run runner, dirs ...string) error {
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
			out, err := run(ctx, dir, s)
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

// execute runs s in dir, its stdout and stderr as one output in the order written.
func execute(ctx context.Context, dir string, s step) (string, error) {
	if _, err := exec.LookPath(s.bin); err != nil {
		if s.install != "" {
			return fmt.Sprintf("%s is not installed: %s", s.bin, s.install), err
		}
		return "", err
	}
	var out bytes.Buffer
	err := cmdutil.New(s.bin, s.args,
		cmdutil.WithContext(ctx),
		cmdutil.WithDir(dir),
		cmdutil.WithStdout(&out),
		cmdutil.WithStderr(&out),
	).Run()
	if err == nil && s.check != nil {
		err = s.check(out.String())
	}
	return out.String(), err
}

// formatted fails on any file gofmt -l lists, generated output and the skill's
// templates aside.
func formatted(output string) error {
	if files := unformatted(output); len(files) > 0 {
		return errors.Newf("not gofmt-clean: %s", strings.Join(files, " "))
	}
	return nil
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
