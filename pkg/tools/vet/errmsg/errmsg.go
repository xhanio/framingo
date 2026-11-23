// Package errmsg reports an xhanio/errors error given no message: one that says
// something failed while withholding what, and a Wrapf layer no reader sees
// (design/errors.md).
package errmsg

import (
	"go/ast"
	"go/constant"
	"go/types"
	"strings"

	"golang.org/x/tools/go/analysis"

	"github.com/xhanio/framingo/pkg/tools/vet/directive"
)

const errorsPath = "github.com/xhanio/errors"

var Analyzer = &analysis.Analyzer{
	Name: "errmsg",
	Doc:  `reports errors.Newf(""), Wrapf(err, ""), WithMessage("") and a bare errors.New() from github.com/xhanio/errors`,
	Run:  run,
}

func run(pass *analysis.Pass) (any, error) {
	for _, file := range pass.Files {
		// a test may build an empty message to see how it behaves
		if strings.HasSuffix(pass.Fset.Position(file.Pos()).Filename, "_test.go") {
			continue
		}
		ast.Inspect(file, func(n ast.Node) bool {
			if call, ok := n.(*ast.CallExpr); ok {
				check(pass, call)
			}
			return true
		})
	}
	return nil, nil
}

func check(pass *analysis.Pass, call *ast.CallExpr) {
	sel, ok := call.Fun.(*ast.SelectorExpr)
	if !ok {
		return
	}
	fn, ok := pass.TypesInfo.Uses[sel.Sel].(*types.Func)
	if !ok || fn.Pkg() == nil || fn.Pkg().Path() != errorsPath {
		return
	}
	method := fn.Type().(*types.Signature).Recv() != nil
	switch fn.Name() {
	case "New":
		// a Category's New() is the category itself, a status-only error
		if !method && len(call.Args) == 0 {
			directive.Reportf(pass, call.Pos(), "errors.New() has no message: say what failed with Newf, or return a bare category for a status alone (design/errors.md)")
		}
	case "Newf", "WithMessage":
		if empty(pass, call, 0) {
			directive.Reportf(pass, call.Pos(), "%s(\"\") has no message: an error that says nothing of what failed is logged as nothing (design/errors.md)", fn.Name())
		}
	case "Wrapf":
		if empty(pass, call, 1) {
			directive.Reportf(pass, call.Pos(), "Wrapf(err, \"\") adds a layer no reader sees: Wrap(err) when there is nothing to add (design/errors.md)")
		}
	}
}

// empty reports whether call's i-th argument is the constant "".
func empty(pass *analysis.Pass, call *ast.CallExpr, i int) bool {
	if len(call.Args) <= i {
		return false
	}
	tv, ok := pass.TypesInfo.Types[call.Args[i]]
	return ok && tv.Value != nil && tv.Value.Kind() == constant.String && constant.StringVal(tv.Value) == ""
}
