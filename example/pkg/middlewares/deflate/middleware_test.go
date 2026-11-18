package deflate

import (
	"bytes"
	"compress/zlib"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/labstack/echo/v4"
	"github.com/stretchr/testify/assert"
)

func deflateOf(t *testing.T, n int) []byte {
	t.Helper()
	var buf bytes.Buffer
	zw := zlib.NewWriter(&buf)
	if _, err := zw.Write(make([]byte, n)); err != nil {
		t.Fatal(err)
	}
	if err := zw.Close(); err != nil {
		t.Fatal(err)
	}
	return buf.Bytes()
}

func funcOf(t *testing.T, m interface {
	Func(bool, []byte) (func(echo.HandlerFunc) echo.HandlerFunc, error)
}) echo.MiddlewareFunc {
	t.Helper()
	fn, err := m.Func(true, nil)
	if err != nil {
		t.Fatal(err)
	}
	return fn
}

func serve(t *testing.T, mw echo.MiddlewareFunc, body []byte) (int64, error) {
	t.Helper()
	e := echo.New()
	req := httptest.NewRequest(http.MethodPost, "/", bytes.NewReader(body))
	req.Header.Set("Content-Encoding", "deflate")
	c := e.NewContext(req, httptest.NewRecorder())

	var read int64
	err := mw(func(c echo.Context) error {
		n, err := io.Copy(io.Discard, c.Request().Body)
		read = n
		return err
	})(c)
	return read, err
}

func TestDecompressionIsBounded(t *testing.T) {
	const limit = 64 << 10
	mw := funcOf(t, New(WithMaxDecompressed(limit)))

	bomb := deflateOf(t, 50<<20)
	t.Logf("compressed %d bytes -> 50 MiB inflated (%.0fx)", len(bomb), float64(50<<20)/float64(len(bomb)))

	read, err := serve(t, mw, bomb)
	if err == nil {
		t.Fatalf("bomb was accepted: handler read %d bytes with no error", read)
	}
	if read > limit+bytes.MinRead {
		t.Fatalf("read %d bytes, well past the %d limit", read, limit)
	}
}

func TestBodyUnderLimitPassesThrough(t *testing.T) {
	payload := bytes.Repeat([]byte("framingo"), 512)
	var buf bytes.Buffer
	zw := zlib.NewWriter(&buf)
	if _, err := zw.Write(payload); err != nil {
		t.Fatal(err)
	}
	zw.Close()

	read, err := serve(t, funcOf(t, New()), buf.Bytes())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if read != int64(len(payload)) {
		t.Fatalf("read %d bytes, want %d", read, len(payload))
	}
}

func TestUncompressedRequestUntouched(t *testing.T) {
	e := echo.New()
	req := httptest.NewRequest(http.MethodPost, "/", bytes.NewReader([]byte("plain")))
	c := e.NewContext(req, httptest.NewRecorder())

	var got []byte
	err := funcOf(t, New())(func(c echo.Context) error {
		b, err := io.ReadAll(c.Request().Body)
		got = b
		return err
	})(c)
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != "plain" {
		t.Fatalf("got %q, want %q", got, "plain")
	}
}

func TestNameDefaultsToPackage(t *testing.T) {
	assert.Equal(t, "deflate", New().Name())
}

func TestWithNameOverrides(t *testing.T) {
	assert.Equal(t, "deflate2", New(WithName("deflate2")).Name())
	assert.Equal(t, "deflate", New(WithName("")).Name())
}
