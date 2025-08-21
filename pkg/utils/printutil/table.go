package printutil

import (
	"fmt"
	"io"
	"reflect"
	"strings"
	"text/tabwriter"
	"time"
	"unicode/utf8"

	"github.com/google/go-cmp/cmp"

	"github.com/xhanio/framingo/pkg/types/common"
	"github.com/xhanio/framingo/pkg/utils/reflectutil"
	"github.com/xhanio/framingo/pkg/utils/sliceutil"
)

const tagKey = "print"

type table struct {
	full bool

	tw *tabwriter.Writer
}

func NewTable(w io.Writer, opts ...Option) Table {
	return newTable(w, opts...)
}

func newTable(w io.Writer, opts ...Option) *table {
	t := &table{
		tw: tabwriter.NewWriter(w, 0, 0, 3, ' ', tabwriter.TabIndent),
	}
	for _, opt := range opts {
		opt(t)
	}
	return t
}

func (t *table) Header(name string) {
	fmt.Fprintf(t.tw, "========== %s ==========\n", name)
	fmt.Fprintln(t.tw)
}

func (t *table) Title(columns ...string) {
	var line string
	for i, column := range columns {
		if i != 0 {
			line += "\t"
		}
		line += "[" + column + "]"
	}
	fmt.Fprintf(t.tw, "%s\n", line)
}

func (t *table) Row(values ...any) {
	var line string
	for i, value := range values {
		if i != 0 {
			line += "\t"
		}
		b := reflectutil.ToBytes(reflect.ValueOf(value))
		if len(b) == 0 {
			line += "-"
		} else {
			switch v := value.(type) {
			case error:
				line += v.Error()
			case time.Time:
				if v.IsZero() {
					line += "-"
				} else {
					line += v.Format(common.TimeFormat)
				}
			default:
				if utf8.Valid(b) {
					s := string(b)
					if !t.full && len(s) > 64 {
						line += s[:64] + "..."
					} else {
						line += s
					}
				} else {
					line += "..."
				}
			}
		}
	}
	fmt.Fprintf(t.tw, "%s\n", line)
}

func (t *table) Diff(a, b any) {
	t.NewLine(cmp.Diff(a, b))
}

func (t *table) Object(obj any) {
	if obj == nil {
		return
	}
	objType := reflect.TypeOf(obj)
	objValue := reflect.ValueOf(obj)
	// fmt.Println("obj type is", objType) //, "value is", objValue
	if objType.Kind() == reflect.Pointer {
		if objValue.IsNil() {
			return
		}
		objType = objType.Elem()
		objValue = objValue.Elem()
		// fmt.Println("obj type is", objType, "value is", objValue)
	}
	if objValue.Kind() != reflect.Struct {
		return
	}
	t.NewLine(objType.Name() + ":")
	t.Title("Field", "Type", "Value")
	for i := 0; i < objType.NumField(); i++ {
		fieldType := objType.Field(i)
		fieldValue := objValue.Field(i)
		tags := strings.Split(fieldType.Tag.Get(tagKey), ",")
		if len(tags) > 0 {
			// fmt.Println("tags are", tags)
			if tags[0] == "-" {
				continue
			}
		}
		fname := fieldType.Name
		ftype := fieldType.Type.Name()
		fvalue := fieldValue.Interface()
		if fieldType.Type.Kind() == reflect.Array {
			if fieldType.Type.Elem().Kind() == reflect.Uint8 { // byte array
				fieldValueSlice := fieldValue.Slice(0, fieldValue.Len())
				ftype = fmt.Sprintf("[%d]byte", fieldValue.Len())
				if sliceutil.In("string", tags...) {
					fvalue = string(fieldValueSlice.Bytes())
				}
			} else { // other array
				ftype = fmt.Sprintf("[%d]%s", fieldValue.Len(), fieldType.Type.Elem().Kind())
			}
		} else if fieldType.Type.Kind() == reflect.Slice {
			if fieldType.Type.Elem().Kind() == reflect.Uint8 { // byte slice
				if sliceutil.In("string", tags...) {
					fvalue = string(fieldValue.Bytes())
				}
			} else { // other slice
				ftype = fmt.Sprintf("[%d]%s", fieldValue.Len(), fieldType.Type.Elem().Kind())
			}
		} else {
			if sliceutil.In("hex", tags...) {
				fvalue = fmt.Sprintf("0x%x", fvalue)
			}
			if sliceutil.In("binary", tags...) {
				fvalue = fmt.Sprintf("0x%b", fvalue)
			}
			if sliceutil.In("redact", tags...) {
				fvalue = common.RedactMask
			}
		}
		t.Row(fname, ftype, fvalue)
	}
}

func (t *table) NewLine(info ...string) {
	fmt.Fprintln(t.tw, strings.Join(info, ""))
}

func (t *table) Flush() {
	t.tw.Flush()
}
