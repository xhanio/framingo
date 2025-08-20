package pathutil

import "path/filepath"

func Short(p string) string {
	return filepath.Join(filepath.Base(filepath.Dir(p)), filepath.Base(p))
}
