package paramutil

import "slices"

type entries[V any] struct {
	params []Param[V]
	index  map[string]int
}

// NewEntries returns an empty Entries.
func NewEntries[V any]() Entries[V] {
	return &entries[V]{index: make(map[string]int)}
}

func (e *entries[V]) Set(key string, value V) {
	if i, ok := e.index[key]; ok {
		e.params[i].Value = value
		return
	}
	e.index[key] = len(e.params)
	e.params = append(e.params, Param[V]{Key: key, Value: value, Keyed: true})
}

func (e *entries[V]) AddRaw(raw string) {
	e.params = append(e.params, Param[V]{Raw: raw})
}

func (e *entries[V]) Get(key string) (V, bool) {
	if i, ok := e.index[key]; ok {
		return e.params[i].Value, true
	}
	var zero V
	return zero, false
}

func (e *entries[V]) Len() int {
	return len(e.params)
}

func (e *entries[V]) List() []Param[V] {
	return slices.Clone(e.params)
}
