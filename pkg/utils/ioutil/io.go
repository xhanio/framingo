package ioutil

import (
	"context"
	"io"

	"github.com/xhanio/errors"
)

func LimitWriteAll(ctx context.Context, w io.Writer, p []byte) (int, error) {
	length := len(p)
	if length == 0 {
		return 0, errors.Wrapf(io.EOF, "nothing to read from")
	}
	offset := 0
	for offset < length {
		select {
		case <-ctx.Done():
			return offset, errors.Cancaled
		default:
			n, err := w.Write(p[offset:])
			if err != nil {
				return offset, err
			}
			offset += n
		}
	}
	return offset, nil
}

func LimitReadAll(ctx context.Context, r io.Reader, p []byte) (int, error) {
	length := len(p)
	if length == 0 {
		return 0, errors.Wrapf(io.EOF, "buffer size must not be zero")
	}
	offset := 0
	for offset < length {
		select {
		case <-ctx.Done():
			return offset, errors.Cancaled
		default:
			n, err := r.Read(p[offset:])
			if err != nil {
				return offset, err
			}
			offset += n
		}
	}
	return offset, nil
}

// copyBuffer is the cancelable implementation of Copy and CopyBuffer.
// if buf is nil, one is allocated.
func CopyBuffer(ctx context.Context, dst io.Writer, src io.Reader, buf []byte) (written int64, err error) {
	if buf == nil {
		size := 32 * 1024
		if l, ok := src.(*io.LimitedReader); ok && int64(size) > l.N {
			if l.N < 1 {
				size = 1
			} else {
				size = int(l.N)
			}
		}
		buf = make([]byte, size)
	}
	for {
		select {
		case <-ctx.Done():
			return written, errors.Cancaled
		default:
			nr, er := src.Read(buf)
			if nr > 0 {
				nw, ew := dst.Write(buf[0:nr])
				if nw < 0 || nr < nw {
					nw = 0
					if ew == nil {
						ew = errors.Newf("invalid write result") // impossible write count
					}
				}
				written += int64(nw)
				if ew != nil {
					return written, ew
				}
				if nr != nw {
					return written, io.ErrShortWrite
				}
			}
			if er != nil {
				if er != io.EOF {
					return written, er
				}
				return written, nil
			}
		}
	}
}
