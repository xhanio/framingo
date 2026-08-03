package nameutil

import (
	"path"
	"strings"

	"github.com/xhanio/framingo/pkg/types/info"
	"github.com/xhanio/framingo/pkg/utils/reflectutil"
)

// Root is the project layout root stripped from every name so the category
// stays visible (services/db/manager, routers/user/router). Projects with a
// different layout may override or clear it.
var Root = "pkg"

// Name returns obj's service name: its package dir relative to the project
// root, with Root stripped, joined with the type name (pkg/services/db +
// manager -> services/db/manager). Optional prefixes are stripped after
// Root, whole segments only, first match wins - Name(m, "services") turns
// services/db/manager into db/manager.
func Name(obj any, strip ...string) string {
	pkg, typeName := reflectutil.Locate(obj)
	dir := trimPrefix(relative(pkg), Root)
	for _, prefix := range strip {
		if trimmed := trimPrefix(dir, prefix); trimmed != dir {
			dir = trimmed
			break
		}
	}
	return path.Join(dir, typeName)
}

// trimPrefix removes a whole-segment prefix; empty or unmatched prefixes
// leave the dir unchanged.
func trimPrefix(dir, prefix string) string {
	if prefix == "" {
		return dir
	}
	if dir == prefix {
		return ""
	}
	if strings.HasPrefix(dir, prefix+"/") {
		return dir[len(prefix)+1:]
	}
	return dir
}

// relative turns a package import path into a project-relative dir by
// cutting it through its overlap with the gopro-injected info.ProjectPath.
// ProjectPath is relative to $GOPATH/src (xhanio/framingo) while import
// paths carry the module host prefix (github.com/xhanio/framingo/pkg/...),
// so the overlap can start mid-path; a dependency's packages may share only
// the leading segments. Without the injection, or without an overlap, the
// import path is returned unchanged.
func relative(pkgPath string) string {
	if pkgPath == "" || info.ProjectPath == "" {
		return pkgPath
	}
	segs := strings.Split(pkgPath, "/")
	proj := strings.Split(info.ProjectPath, "/")
	// longest contiguous run of project segments found segment-aligned in
	// the import path; ties go to the rightmost, cutting the most. A single
	// shared segment counts only when it is the entire ProjectPath - an org
	// name shared with an unrelated dependency must not cut it.
	cut, best := 0, 0
	for i := range segs {
		for j := range proj {
			n := 0
			for i+n < len(segs) && j+n < len(proj) && segs[i+n] == proj[j+n] {
				n++
			}
			if n > 1 || n == len(proj) {
				if n > best || (n == best && i+n > cut) {
					best, cut = n, i+n
				}
			}
		}
	}
	return strings.Join(segs[cut:], "/")
}
