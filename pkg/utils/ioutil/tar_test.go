package ioutil

import (
	"archive/tar"
	"compress/gzip"
	"context"
	"io"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/xhanio/errors"
)

func srcDir(t *testing.T) string {
	t.Helper()
	src := t.TempDir()
	require.NoError(t, os.MkdirAll(filepath.Join(src, "sub"), 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(src, "a.txt"), []byte("alpha"), 0o644))
	require.NoError(t, os.WriteFile(filepath.Join(src, "sub", "b.txt"), []byte("beta"), 0o644))
	return src
}

func TestTargzRoundTrips(t *testing.T) {
	dst := filepath.Join(t.TempDir(), "out.tar.gz")
	require.NoError(t, Targz(context.Background(), srcDir(t), dst))

	f, err := os.Open(dst)
	require.NoError(t, err)
	defer f.Close()
	gzr, err := gzip.NewReader(f)
	require.NoError(t, err)
	files := map[string]string{}
	tr := tar.NewReader(gzr)
	for {
		h, err := tr.Next()
		if errors.Is(err, io.EOF) {
			break
		}
		require.NoError(t, err)
		data, err := io.ReadAll(tr)
		require.NoError(t, err)
		files[h.Name] = string(data)
	}
	assert.Equal(t, map[string]string{"a.txt": "alpha", filepath.Join("sub", "b.txt"): "beta"}, files)
}

// fullAfter takes n bytes, then fails every write.
type fullAfter struct{ n int }

func (w *fullAfter) Write(p []byte) (int, error) {
	if len(p) > w.n {
		return 0, io.ErrShortWrite
	}
	w.n -= len(p)
	return len(p), nil
}

// Small files sit in gzip's buffer until the writers close: a destination
// that fills after gzip's header fails only there, and the archive with it.
func TestTargzReportsAFailedClose(t *testing.T) {
	const gzipHeader = 10
	assert.Error(t, writeTargz(context.Background(), srcDir(t), &fullAfter{n: gzipHeader}))
}

func TestCopyFile(t *testing.T) {
	src := srcDir(t)
	dst := filepath.Join(t.TempDir(), "copy.txt")
	require.NoError(t, CopyFile(context.Background(), filepath.Join(src, "a.txt"), dst))
	data, err := os.ReadFile(dst)
	require.NoError(t, err)
	assert.Equal(t, "alpha", string(data))
}
