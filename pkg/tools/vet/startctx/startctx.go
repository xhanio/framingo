// Package startctx reports a Start method that keeps its context: what a
// start runs lives until Stop, so it may keep the context's values but never
// its cancel or deadline (design/lifecycle.md).
package startctx

import (
	"go/ast"
	"go/types"

	"golang.org/x/tools/go/analysis"

	"github.com/xhanio/framingo/pkg/tools/vet/directive"
)

const advice = "what a start runs lives until Stop: derive from context.WithoutCancel(ctx), or run it on a syncutil.Loop (design/lifecycle.md)"

var Analyzer = &analysis.Analyzer{
	Name: "startctx",
	Doc:  "reports a Start(ctx) method that keeps its context, or one derived from it, past the call",
	Run:  run,
}

func run(pass *analysis.Pass) (any, error) {
	for _, file := range pass.Files {
		for _, decl := range file.Decls {
			fd, ok := decl.(*ast.FuncDecl)
			if !ok || fd.Body == nil {
				continue
			}
			if param := startParam(pass, fd); param != nil {
				check(pass, fd.Body, param)
			}
		}
	}
	return nil, nil
}

// startParam is the context parameter of a method Start(ctx context.Context) error, if fd is one.
func startParam(pass *analysis.Pass, fd *ast.FuncDecl) types.Object {
	if fd.Recv == nil || fd.Name.Name != "Start" {
		return nil
	}
	params, results := fd.Type.Params, fd.Type.Results
	if params == nil || len(params.List) != 1 || len(params.List[0].Names) != 1 || results == nil || len(results.List) != 1 {
		return nil
	}
	name := params.List[0].Names[0]
	obj := pass.TypesInfo.Defs[name]
	if obj == nil || !isNamed(obj.Type(), "context", "Context") || !isNamed(pass.TypesInfo.TypeOf(results.List[0].Type), "", "error") {
		return nil
	}
	return obj
}

func isNamed(t types.Type, pkg, name string) bool {
	named, ok := t.(*types.Named)
	if !ok || named.Obj().Name() != name {
		return false
	}
	if named.Obj().Pkg() == nil {
		return pkg == ""
	}
	return named.Obj().Pkg().Path() == pkg
}

// check follows the start's context through the body: the parameter, the
// contexts derived from it and their cancel funcs are "kept" once stored in a
// field or used by a goroutine the method starts.
func check(pass *analysis.Pass, body *ast.BlockStmt, param types.Object) {
	bound := map[types.Object]bool{param: true} // carries the start's cancel or deadline

	isBound := func(e ast.Expr) bool {
		id, ok := ast.Unparen(e).(*ast.Ident)
		return ok && bound[pass.TypesInfo.Uses[id]]
	}
	// derives reports whether call is context.WithCancel, WithTimeout, WithDeadline
	// or WithValue of a bound context: its results are bound as well.
	derives := func(e ast.Expr) bool {
		call, ok := ast.Unparen(e).(*ast.CallExpr)
		if !ok || len(call.Args) == 0 {
			return false
		}
		sel, ok := call.Fun.(*ast.SelectorExpr)
		if !ok {
			return false
		}
		fn, ok := pass.TypesInfo.Uses[sel.Sel].(*types.Func)
		if !ok || fn.Pkg() == nil || fn.Pkg().Path() != "context" {
			return false
		}
		switch fn.Name() {
		case "WithCancel", "WithCancelCause", "WithTimeout", "WithTimeoutCause", "WithDeadline", "WithDeadlineCause", "WithValue":
			return isBound(call.Args[0])
		}
		return false
	}
	store := func(lhs ast.Expr, what string) {
		switch l := ast.Unparen(lhs).(type) {
		case *ast.Ident:
			if obj := pass.TypesInfo.ObjectOf(l); obj != nil {
				bound[obj] = true
			}
		case *ast.SelectorExpr:
			directive.Reportf(pass, lhs.Pos(), "Start keeps %s in %s: %s", what, types.ExprString(lhs), advice)
		}
	}

	// a variable assigned anything else no longer carries the start's cancel
	release := func(lhs ast.Expr) {
		if id, ok := ast.Unparen(lhs).(*ast.Ident); ok {
			delete(bound, pass.TypesInfo.ObjectOf(id))
		}
	}

	ast.Inspect(body, func(n ast.Node) bool {
		switch n := n.(type) {
		case *ast.AssignStmt:
			if len(n.Rhs) == 1 && derives(n.Rhs[0]) {
				store(n.Lhs[0], "a context derived from its ctx")
				if len(n.Lhs) > 1 {
					store(n.Lhs[1], "the cancel of a context derived from its ctx")
				}
				return true
			}
			if len(n.Lhs) == len(n.Rhs) {
				for i, rhs := range n.Rhs {
					if isBound(rhs) {
						store(n.Lhs[i], "its ctx")
					} else {
						release(n.Lhs[i])
					}
				}
			} else {
				for _, lhs := range n.Lhs {
					release(lhs)
				}
			}
		case *ast.GoStmt:
			ast.Inspect(n.Call, func(m ast.Node) bool {
				if id, ok := m.(*ast.Ident); ok && bound[pass.TypesInfo.Uses[id]] {
					directive.Reportf(pass, id.Pos(), "Start hands its ctx to a goroutine it leaves running: %s", advice)
				}
				return true
			})
			return false
		}
		return true
	})
}
