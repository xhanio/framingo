package ioutil

import (
	"archive/tar"
	"compress/gzip"
	"context"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/xhanio/errors"
)

// Targz writes the regular files under src into dst.
func Targz(ctx context.Context, src, dst string) error {
	if _, err := os.Stat(src); err != nil {
		return errors.Wrapf(err, "unable to tar %s", src)
	}

	w, err := os.Create(dst)
	if err != nil {
		return errors.Wrap(err)
	}
	err = writeTargz(ctx, src, w)
	return errors.Combine(err, w.Close())
}

// writeTargz closes each writer into the next: small files sit in gzip's
// buffer until then, so a failed close fails the archive.
func writeTargz(ctx context.Context, src string, w io.Writer) error {
	gzw := gzip.NewWriter(w)
	tw := tar.NewWriter(gzw)

	err := filepath.Walk(src, func(file string, fi os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if !fi.Mode().IsRegular() {
			return nil
		}
		header, err := tar.FileInfoHeader(fi, fi.Name())
		if err != nil {
			return err
		}
		header.Name = strings.TrimPrefix(strings.ReplaceAll(file, src, ""), string(filepath.Separator))
		if err := tw.WriteHeader(header); err != nil {
			return err
		}
		return copyInto(ctx, tw, file)
	})
	return errors.Combine(err, tw.Close(), gzw.Close())
}

func copyInto(ctx context.Context, w io.Writer, file string) error {
	f, err := os.Open(file)
	if err != nil {
		return err
	}
	//nolint:errcheck // read only: nothing a failed close could lose
	defer f.Close()
	_, err = CopyBuffer(ctx, w, f, nil)
	return err
}
