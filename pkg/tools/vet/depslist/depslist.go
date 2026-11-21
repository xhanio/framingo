// Package depslist reports a service that holds another service in a field
// and leaves it out of Dependencies, and a Dependencies that filters what it
// lists: the supervisor orders, gates and monitors by that list, so what is
// not in it starts in no particular order, and it skips a nil entry itself
// (design/service-wiring.md).
package depslist

import (
	"go/ast"
	"go/types"
	"strings"

	"golang.org/x/tools/go/analysis"

	"github.com/xhanio/framingo/pkg/tools/vet/directive"
)

var Analyzer = &analysis.Analyzer{
	Name: "depslist",
	Doc:  "reports a field holding a service that the type's Dependencies method does not list, and a condition in Dependencies",
	Run:  run,
}

func run(pass *analysis.Pass) (any, error) {
	for _, file := range pass.Files {
		for _, decl := range file.Decls {
			fd, ok := decl.(*ast.FuncDecl)
			if !ok || fd.Body == nil {
				continue
			}
			// a test's double embeds what it fakes
			if strings.HasSuffix(pass.Fset.Position(fd.Pos()).Filename, "_test.go") {
				continue
			}
			if recv, st, service := dependencies(pass, fd); st != nil {
				check(pass, fd, recv, st, service)
			}
		}
	}
	return nil, nil
}

// dependencies is fd's receiver variable, its struct and the service
// interface, if fd is a method Dependencies() []S on a struct, S an interface.
func dependencies(pass *analysis.Pass, fd *ast.FuncDecl) (types.Object, *types.Struct, *types.Interface) {
	if fd.Recv == nil || fd.Name.Name != "Dependencies" || len(fd.Recv.List) != 1 {
		return nil, nil, nil
	}
	fn, ok := pass.TypesInfo.Defs[fd.Name].(*types.Func)
	if !ok {
		return nil, nil, nil
	}
	sig := fn.Type().(*types.Signature)
	if sig.Params().Len() != 0 || sig.Results().Len() != 1 {
		return nil, nil, nil
	}
	slice, ok := sig.Results().At(0).Type().(*types.Slice)
	if !ok {
		return nil, nil, nil
	}
	service, ok := slice.Elem().Underlying().(*types.Interface)
	if !ok || service.NumMethods() == 0 {
		return nil, nil, nil
	}
	recvType := sig.Recv().Type()
	if p, ok := recvType.(*types.Pointer); ok {
		recvType = p.Elem()
	}
	st, ok := recvType.Underlying().(*types.Struct)
	if !ok {
		return nil, nil, nil
	}
	var recv types.Object
	if names := fd.Recv.List[0].Names; len(names) == 1 {
		recv = pass.TypesInfo.Defs[names[0]]
	}
	return recv, st, service
}

func check(pass *analysis.Pass, fd *ast.FuncDecl, recv types.Object, st *types.Struct, service *types.Interface) {
	ast.Inspect(fd.Body, func(n ast.Node) bool {
		switch n.(type) {
		case *ast.IfStmt, *ast.SwitchStmt, *ast.TypeSwitchStmt, *ast.TypeAssertExpr:
			directive.Reportf(pass, n.Pos(), "Dependencies filters what it lists: list what is held as it is, the supervisor skips a nil entry (design/service-wiring.md)")
		case *ast.FuncLit:
			return false
		}
		return true
	})
	listed := map[types.Object]bool{}
	if recv != nil {
		ast.Inspect(fd.Body, func(n ast.Node) bool {
			sel, ok := n.(*ast.SelectorExpr)
			if !ok {
				return true
			}
			if id, ok := ast.Unparen(sel.X).(*ast.Ident); ok && pass.TypesInfo.Uses[id] == recv {
				listed[pass.TypesInfo.Uses[sel.Sel]] = true
			}
			return true
		})
	}
	for i := range st.NumFields() {
		f := st.Field(i)
		if listed[f] || !holdsService(f.Type(), service) {
			continue
		}
		directive.Reportf(pass, f.Pos(), "field %s holds a service that Dependencies does not list: the supervisor orders and monitors by that list, nil entries skipped (design/service-wiring.md)", f.Name())
	}
}

func holdsService(t types.Type, service *types.Interface) bool {
	if types.Implements(t, service) {
		return true
	}
	if _, isPtr := t.(*types.Pointer); !isPtr && !types.IsInterface(t) {
		return types.Implements(types.NewPointer(t), service)
	}
	return false
}
