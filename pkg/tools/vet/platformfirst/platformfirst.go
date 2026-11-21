// Package platformfirst reports an app service whose New does not take the
// platform first: one constructor shape across the app, so a service that
// comes to need a handle changes neither its signature nor the wiring
// (design/service-wiring.md).
package platformfirst

import (
	"go/ast"
	"go/types"
	"strings"

	"golang.org/x/tools/go/analysis"

	"github.com/xhanio/framingo/pkg/tools/vet/directive"
)

// the framework's own services take no platform: they cannot know an app's bundle
const framework = "github.com/xhanio/framingo/pkg/"

var Analyzer = &analysis.Analyzer{
	Name: "platformfirst",
	Doc:  "reports an app service's New whose first parameter is not the platform",
	Run:  run,
}

func run(pass *analysis.Pass) (any, error) {
	path := pass.Pkg.Path()
	if !strings.Contains(path+"/", "/pkg/services/") || strings.HasPrefix(path, framework) {
		return nil, nil
	}
	for _, file := range pass.Files {
		for _, decl := range file.Decls {
			fd, ok := decl.(*ast.FuncDecl)
			if !ok || fd.Recv != nil || fd.Name.Name != "New" {
				continue
			}
			sig := pass.TypesInfo.Defs[fd.Name].Type().(*types.Signature)
			// what builds the platform takes its handles instead
			if sig.Results().Len() > 0 && named(sig.Results().At(0).Type()) == "Platform" {
				continue
			}
			if sig.Params().Len() == 0 || named(sig.Params().At(0).Type()) != "Platform" {
				directive.Reportf(pass, fd.Name.Pos(), "New does not take the platform first: every app service's constructor is New(platform, ...), whether it uses it yet or not (design/service-wiring.md)")
			}
		}
	}
	return nil, nil
}

func named(t types.Type) string {
	if n, ok := t.(*types.Named); ok {
		return n.Obj().Name()
	}
	return ""
}
