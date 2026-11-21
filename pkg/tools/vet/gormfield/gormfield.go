// Package gormfield reports a *gorm.DB kept in a struct field outside the db
// service: a dependent keeps the database's handle and asks it each time, so a
// restart that replaces the pool is invisible to it (design/service-wiring.md).
package gormfield

import (
	"go/ast"
	"go/types"
	"strings"

	"golang.org/x/tools/go/analysis"

	"github.com/xhanio/framingo/pkg/tools/vet/directive"
)

var Analyzer = &analysis.Analyzer{
	Name: "gormfield",
	Doc:  "reports a *gorm.DB kept in a struct field outside the db service",
	Run:  run,
}

func run(pass *analysis.Pass) (any, error) {
	// the db service owns the pool, and swaps it whole
	if strings.Contains(pass.Pkg.Path()+"/", "/pkg/services/db/") {
		return nil, nil
	}
	for _, file := range pass.Files {
		ast.Inspect(file, func(n ast.Node) bool {
			st, ok := n.(*ast.StructType)
			if !ok {
				return true
			}
			for _, f := range st.Fields.List {
				if isGormDB(pass.TypesInfo.TypeOf(f.Type)) {
					directive.Reportf(pass, f.Pos(), "a *gorm.DB kept in a field goes on using a pool a restart closed: keep the database handle and call FromContext(ctx) for each query (design/service-wiring.md)")
				}
			}
			return true
		})
	}
	return nil, nil
}

func isGormDB(t types.Type) bool {
	p, ok := t.(*types.Pointer)
	if !ok {
		return false
	}
	n, ok := p.Elem().(*types.Named)
	return ok && n.Obj().Name() == "DB" && n.Obj().Pkg() != nil && n.Obj().Pkg().Path() == "gorm.io/gorm"
}
