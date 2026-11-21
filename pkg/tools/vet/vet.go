// Package vet runs the analyzers that hold a project to the framework's
// design decisions, each in a subpackage named for its rule. go vet does the
// running, with this binary as its -vettool, so results are cached per
// package as go vet's own are.
package vet

import (
	"strings"

	"golang.org/x/tools/go/analysis"
	"golang.org/x/tools/go/analysis/unitchecker"

	"github.com/xhanio/framingo/pkg/tools/vet/confinit"
	"github.com/xhanio/framingo/pkg/tools/vet/depslist"
	"github.com/xhanio/framingo/pkg/tools/vet/gormfield"
	"github.com/xhanio/framingo/pkg/tools/vet/optionclass"
	"github.com/xhanio/framingo/pkg/tools/vet/platformfirst"
	"github.com/xhanio/framingo/pkg/tools/vet/startctx"
)

func Analyzers() []*analysis.Analyzer {
	return []*analysis.Analyzer{
		confinit.Analyzer,
		depslist.Analyzer,
		gormfield.Analyzer,
		optionclass.Analyzer,
		platformfirst.Analyzer,
		startctx.Analyzer,
	}
}

// AsTool reports whether args are go vet invoking this binary as its -vettool:
// it asks for the version and the flags, then passes one .cfg file per package.
func AsTool(args []string) bool {
	for _, arg := range args {
		if strings.HasPrefix(arg, "-V") || arg == "-flags" || strings.HasSuffix(arg, ".cfg") {
			return true
		}
	}
	return false
}

// Main serves go vet as its -vettool; it does not return.
func Main() {
	unitchecker.Main(Analyzers()...)
}

// Args are go's arguments to vet patterns (default ./...) with the binary at
// self - this one - as its tool: exec "go" with them.
func Args(self string, patterns ...string) []string {
	if len(patterns) == 0 {
		patterns = []string{"./..."}
	}
	return append([]string{"vet", "-vettool=" + self}, patterns...)
}
