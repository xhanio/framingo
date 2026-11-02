package ioutil

import (
	"strings"
	"sync"
	"testing"
)

func TestTailWriterKeepsLastBytes(t *testing.T) {
	tests := []struct {
		name   string
		max    int
		writes []string
		want   string
	}{
		{"under the limit", 5, []string{"ab"}, "ab"},
		{"exactly the limit", 5, []string{"abcde"}, "abcde"},
		{"one byte over", 5, []string{"abcde", "f"}, "bcdef"},
		{"spanning writes", 5, []string{"abc", "defg"}, "cdefg"},
		{"one write longer than the limit", 5, []string{"abcdefgh"}, "defgh"},
		{"long write after wrapping", 5, []string{"abc", "defg", "hijklmn"}, "jklmn"},
		{"wrapping many times", 3, strings.Split("abcdefghijklmnopqrstuvwxyz", ""), "xyz"},
		{"empty writes", 3, []string{"", "ab", ""}, "ab"},
		{"zero limit keeps nothing", 0, []string{"abc"}, ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			w := NewTailWriter(tt.max)
			for _, s := range tt.writes {
				n, err := w.Write([]byte(s))
				if err != nil || n != len(s) {
					t.Fatalf("Write(%q) = %d, %v; want %d, nil", s, n, err, len(s))
				}
			}
			if got := w.String(); got != tt.want {
				t.Fatalf("String() = %q, want %q", got, tt.want)
			}
			if got := string(w.Bytes()); got != tt.want {
				t.Fatalf("Bytes() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestTailWriterBytesIsACopy(t *testing.T) {
	w := NewTailWriter(4)
	_, _ = w.Write([]byte("abcd"))
	b := w.Bytes()
	_, _ = w.Write([]byte("ef"))
	if string(b) != "abcd" {
		t.Fatalf("later writes changed an earlier Bytes() result to %q", b)
	}
}

func TestTailWriterConcurrentWrites(t *testing.T) {
	w := NewTailWriter(64)
	var wg sync.WaitGroup
	for range 8 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for range 100 {
				_, _ = w.Write([]byte("0123456789"))
				_ = w.String()
			}
		}()
	}
	wg.Wait()
	if n := len(w.Bytes()); n != 64 {
		t.Fatalf("kept %d bytes, want 64", n)
	}
}
