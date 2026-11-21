// Package confinit reports configuration read from the context outside Init:
// a service reads what it can reload in Init, which runs at start and on
// every restart, so a restart is the reload (design/config.md).
package confinit

import (
	"go/ast"
	"go/types"
	"strings"

	"golang.org/x/tools/go/analysis"

	"github.com/xhanio/framingo/pkg/tools/vet/directive"
)

var Analyzer = &analysis.Analyzer{
	Name: "confinit",
	Doc:  "reports confutil.FromContext called outside an Init method",
	Run:  run,
}

func run(pass *analysis.Pass) (any, error) {
	for _, file := range pass.Files {
		// a test reads the config it wrapped to assert on it
		if strings.HasSuffix(pass.Fset.Position(file.Pos()).Filename, "_test.go") {
			continue
		}
		for _, decl := range file.Decls {
			fd, ok := decl.(*ast.FuncDecl)
			if !ok || fd.Body == nil || (fd.Recv != nil && fd.Name.Name == "Init") {
				continue
			}
			ast.Inspect(fd.Body, func(n ast.Node) bool {
				call, ok := n.(*ast.CallExpr)
				if ok && isFromContext(pass, call) {
					directive.Reportf(pass, call.Pos(), "confutil.FromContext outside Init: read what the service can reload in Init, which a restart reruns, and keep it on the manager (design/config.md)")
				}
				return true
			})
		}
	}
	return nil, nil
}

func isFromContext(pass *analysis.Pass, call *ast.CallExpr) bool {
	sel, ok := call.Fun.(*ast.SelectorExpr)
	if !ok {
		return false
	}
	fn, ok := pass.TypesInfo.Uses[sel.Sel].(*types.Func)
	return ok && fn.Name() == "FromContext" && fn.Pkg() != nil && strings.HasSuffix(fn.Pkg().Path(), "/pkg/utils/confutil")
}
