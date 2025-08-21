package printutil

import (
	"os"
	"testing"
)

func TestTable(t *testing.T) {
	tt := newTable(os.Stdout)
	tt.Title("a", "b", "c")
	tt.Row("1", "1", "1")
	tt.Flush()
}
