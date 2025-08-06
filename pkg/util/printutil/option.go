package printutil

type Option func(*table)

func FullDisplay() Option {
	return func(t *table) {
		t.full = true
	}
}
