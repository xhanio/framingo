package ioutil

import (
	"context"
	"os"
	"path/filepath"

	"github.com/xhanio/errors"
)

func CopyFile(ctx context.Context, src, dst string) error {
	r, err := os.Open(src)
	if err != nil {
		return errors.Wrap(err)
	}
	defer r.Close()

	w, err := os.Create(dst)
	if err != nil {
		return errors.Wrap(err)
	}
	defer w.Close()

	_, err = CopyBuffer(ctx, w, r, nil)
	if err != nil {
		return errors.Wrap(err)
	}
	return nil
}

func CopyDir(ctx context.Context, source string, dest string) (err error) {
	// get properties of source dir
	sourceinfo, err := os.Stat(source)
	if err != nil {
		return err
	}

	// create dest dir
	err = os.MkdirAll(dest, sourceinfo.Mode())
	if err != nil {
		return err
	}

	directory, err := os.Open(source)
	if err != nil {
		return err
	}
	defer directory.Close()

	objects, err := directory.Readdir(-1)
	if err != nil {
		return err
	}

	for _, obj := range objects {
		// Check if context has been cancelled
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
			sourcePath := filepath.Join(source, obj.Name())
			destPath := filepath.Join(dest, obj.Name())

			if obj.IsDir() {
				err = CopyDir(ctx, sourcePath, destPath)
				if err != nil {
					return err
				}
			} else {
				err = CopyFile(ctx, sourcePath, destPath)
				if err != nil {
					return err
				}
			}
		}
	}
	return nil
}
