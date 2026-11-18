package nameutil

import (
	"path"
	"strings"

	"github.com/xhanio/framingo/pkg/types/info"
	"github.com/xhanio/framingo/pkg/utils/reflectutil"
)

// Root is stripped from every name so the category leads (services/db/manager); may be overridden or cleared.
var Root = "pkg"

func Package(obj any) string {
	pkg, _ := reflectutil.Locate(obj)
	return path.Base(pkg)
}

// Name returns the project-relative package dir minus Root, joined with the type name
// (services/db/manager); strip removes the first matching whole-segment prefix.
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

// relative cuts pkgPath at its overlap with info.ProjectPath, which lacks the module host
// prefix so the overlap can start mid-path; without ProjectPath or an overlap it is unchanged.
func relative(pkgPath string) string {
	if pkgPath == "" || info.ProjectPath == "" {
		return pkgPath
	}
	segs := strings.Split(pkgPath, "/")
	proj := strings.Split(info.ProjectPath, "/")
	// longest aligned run, ties to the rightmost; a single segment counts only if it is the
	// whole ProjectPath, so a shared org name in a dependency does not cut
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

// ConfigRoot is the package segment of a pathed name (services/db/manager -> db) or the name
// itself when unpathed; never a slashed name, which an env override cannot spell.
func ConfigRoot(name string) string {
	dir := path.Dir(name)
	if dir == "." || dir == "/" {
		return name
	}
	return path.Base(dir)
}
