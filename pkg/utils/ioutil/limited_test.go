package ioutil

import (
	"bytes"
	"io"
	"strings"
	"testing"
)

func TestLimitReaderUnderLimit(t *testing.T) {
	lr := NewLimitReader(strings.NewReader("hello"), 10)
	got, err := io.ReadAll(lr)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if string(got) != "hello" {
		t.Fatalf("got %q, want %q", got, "hello")
	}
	if lr.Consumed() != 5 {
		t.Fatalf("Consumed() = %d, want 5", lr.Consumed())
	}
	if lr.Remaining() != 5 {
		t.Fatalf("Remaining() = %d, want 5", lr.Remaining())
	}
}

func TestLimitReaderExactlyAtLimit(t *testing.T) {
	lr := NewLimitReader(strings.NewReader("hello"), 5)
	got, err := io.ReadAll(lr)
	if err != nil {
		t.Fatalf("exact-limit read should succeed, got %v", err)
	}
	if string(got) != "hello" {
		t.Fatalf("got %q, want %q", got, "hello")
	}
	if lr.Remaining() != 0 {
		t.Fatalf("Remaining() = %d, want 0", lr.Remaining())
	}
}

func TestLimitReaderOverLimitFails(t *testing.T) {
	lr := NewLimitReader(strings.NewReader("hello world"), 5)
	if _, err := io.ReadAll(lr); err == nil {
		t.Fatal("expected an error past the limit, got nil")
	}
}

func TestLimitReaderDoesNotTruncateSilently(t *testing.T) {
	lr := NewLimitReader(&iotest{data: bytes.Repeat([]byte("x"), 100)}, 10)
	n, err := io.Copy(io.Discard, lr)
	if err == nil {
		t.Fatalf("expected failure, copied %d bytes cleanly", n)
	}
	if n > 10 {
		t.Fatalf("copied %d bytes, past the limit of 10", n)
	}
}

func TestLimitReaderZeroLimit(t *testing.T) {
	lr := NewLimitReader(strings.NewReader("x"), 0)
	if _, err := io.ReadAll(lr); err == nil {
		t.Fatal("expected an error with a zero limit and non-empty input")
	}
}

func TestLimitReaderCloseDelegates(t *testing.T) {
	rc := &closeSpy{Reader: strings.NewReader("hello")}
	lr := NewLimitReader(rc, 10)
	if err := lr.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}
	if !rc.closed {
		t.Fatal("Close did not reach the wrapped reader")
	}
}

func TestLimitReaderCloseOnNonCloser(t *testing.T) {
	if err := NewLimitReader(strings.NewReader("hello"), 10).Close(); err != nil {
		t.Fatalf("Close on a non-Closer should be a no-op, got %v", err)
	}
}

type closeSpy struct {
	io.Reader
	closed bool
}

func (c *closeSpy) Close() error { c.closed = true; return nil }

// iotest yields one byte per Read.
type iotest struct {
	data []byte
	off  int
}

func (r *iotest) Read(p []byte) (int, error) {
	if r.off >= len(r.data) {
		return 0, io.EOF
	}
	if len(p) == 0 {
		return 0, nil
	}
	p[0] = r.data[r.off]
	r.off++
	return 1, nil
}

func TestLimitWriterReportsATruncatedWriteAsWhole(t *testing.T) {
	var kept, all bytes.Buffer
	// io.MultiWriter gives up on a short count, so a truncating write must not report one
	w := io.MultiWriter(NewLimitWriter(&kept, 4), &all)
	n, err := w.Write([]byte("abcdefgh"))
	if err != nil {
		t.Fatalf("Write = %v", err)
	}
	if n != 8 {
		t.Fatalf("n = %d, want 8", n)
	}
	if got := kept.String(); got != "abcd" {
		t.Fatalf("kept = %q, want %q", got, "abcd")
	}
	if got := all.String(); got != "abcdefgh" {
		t.Fatalf("all = %q, want %q", got, "abcdefgh")
	}
}
