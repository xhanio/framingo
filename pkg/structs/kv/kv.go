// Package kv provides an ordered list of key/value entries in which a key is
// written at most once, so one set of values can be overlaid on another without
// disturbing the order the first one established.
package kv

import "slices"

type list[V any] struct {
	entries []Entry[V]
	index   map[string]int
}

// New returns an empty List.
func New[V any]() List[V] {
	return &list[V]{index: make(map[string]int)}
}

func (l *list[V]) Set(key string, value V) {
	if i, ok := l.index[key]; ok {
		l.entries[i].Value = value
		return
	}
	l.index[key] = len(l.entries)
	l.entries = append(l.entries, Entry[V]{Key: key, Value: value, Keyed: true})
}

func (l *list[V]) AddRaw(raw string) {
	l.entries = append(l.entries, Entry[V]{Raw: raw})
}

func (l *list[V]) Get(key string) (V, bool) {
	if i, ok := l.index[key]; ok {
		return l.entries[i].Value, true
	}
	var zero V
	return zero, false
}

func (l *list[V]) Len() int {
	return len(l.entries)
}

func (l *list[V]) Entries() []Entry[V] {
	return slices.Clone(l.entries)
}
