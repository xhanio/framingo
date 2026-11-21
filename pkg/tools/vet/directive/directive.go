// Package directive lets a line opt out of one analyzer, with its reason:
//
//	//framingo:ignore depslist a handle given to one connection, never supervised
//
// on the reported line or the line above it. Without a reason it is not honoured.
package directive

import (
	"go/token"
	"strings"

	"golang.org/x/tools/go/analysis"
)

const prefix = "//framingo:ignore "

// Reportf reports unless a directive naming pass's analyzer covers pos.
func Reportf(pass *analysis.Pass, pos token.Pos, format string, args ...any) {
	if !ignored(pass, pos) {
		pass.Reportf(pos, format, args...)
	}
}

func ignored(pass *analysis.Pass, pos token.Pos) bool {
	line := pass.Fset.Position(pos).Line
	for _, file := range pass.Files {
		if pos < file.FileStart || pos > file.FileEnd {
			continue
		}
		for _, group := range file.Comments {
			for _, c := range group.List {
				if !strings.HasPrefix(c.Text, prefix) {
					continue
				}
				fields := strings.Fields(strings.TrimPrefix(c.Text, prefix))
				if len(fields) < 2 || fields[0] != pass.Analyzer.Name {
					continue
				}
				if at := pass.Fset.Position(c.Pos()).Line; at == line || at == line-1 {
					return true
				}
			}
		}
	}
	return false
}
