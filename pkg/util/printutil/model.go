package printutil

type Table interface {
	Header(name string)
	Title(columns ...string)
	Row(values ...any)
	Object(obj any)
	NewLine(info ...string)
	Flush()
}
