package kv_test

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/xhanio/framingo/pkg/structs/kv"
)

func TestList_Set(t *testing.T) {
	t.Run("appends new keys in the order they are set", func(t *testing.T) {
		l := kv.New[string]()
		l.Set("a", "1")
		l.Set("b", "2")
		l.Set("c", "3")
		assert.Equal(t, []kv.Entry[string]{
			{Key: "a", Value: "1", Keyed: true},
			{Key: "b", Value: "2", Keyed: true},
			{Key: "c", Value: "3", Keyed: true},
		}, l.Entries())
	})

	t.Run("overwrites a known key in place", func(t *testing.T) {
		l := kv.New[string]()
		l.Set("a", "1")
		l.Set("b", "2")
		l.Set("a", "3")
		assert.Equal(t, []kv.Entry[string]{
			{Key: "a", Value: "3", Keyed: true},
			{Key: "b", Value: "2", Keyed: true},
		}, l.Entries())
	})

	t.Run("treats the empty string as a key", func(t *testing.T) {
		l := kv.New[string]()
		l.Set("", "1")
		l.Set("", "2")
		assert.Equal(t, []kv.Entry[string]{{Value: "2", Keyed: true}}, l.Entries())
	})
}

func TestList_AddRaw(t *testing.T) {
	t.Run("keeps unkeyed entries in the position they were added", func(t *testing.T) {
		l := kv.New[string]()
		l.Set("a", "1")
		l.AddRaw("x")
		l.Set("b", "2")
		assert.Equal(t, []kv.Entry[string]{
			{Key: "a", Value: "1", Keyed: true},
			{Raw: "x"},
			{Key: "b", Value: "2", Keyed: true},
		}, l.Entries())
	})

	t.Run("never merges unkeyed entries with each other", func(t *testing.T) {
		l := kv.New[string]()
		l.AddRaw("x")
		l.AddRaw("x")
		assert.Len(t, l.Entries(), 2)
	})

	t.Run("does not let an unkeyed entry answer a Get", func(t *testing.T) {
		l := kv.New[string]()
		l.AddRaw("a")
		_, ok := l.Get("a")
		assert.False(t, ok)
	})
}

func TestList_Get(t *testing.T) {
	t.Run("returns the last value set", func(t *testing.T) {
		l := kv.New[string]()
		l.Set("a", "1")
		l.Set("a", "2")
		got, ok := l.Get("a")
		assert.True(t, ok)
		assert.Equal(t, "2", got)
	})

	t.Run("reports a key that was never set", func(t *testing.T) {
		l := kv.New[string]()
		got, ok := l.Get("a")
		assert.False(t, ok)
		assert.Empty(t, got)
	})

	t.Run("hands back a stored pointer so callers can mutate through it", func(t *testing.T) {
		l := kv.New[*[]string]()
		values := []string{"1"}
		l.Set("a", &values)
		got, ok := l.Get("a")
		assert.True(t, ok)
		*got = append(*got, "2")
		assert.Equal(t, []string{"1", "2"}, *l.Entries()[0].Value)
	})
}

func TestList_Len(t *testing.T) {
	l := kv.New[string]()
	assert.Equal(t, 0, l.Len())
	l.Set("a", "1")
	l.AddRaw("x")
	l.Set("a", "2")
	assert.Equal(t, 2, l.Len())
}

func TestList_Entries(t *testing.T) {
	t.Run("returns nothing for a list never written to", func(t *testing.T) {
		l := kv.New[string]()
		assert.Empty(t, l.Entries())
	})

	t.Run("does not alias the list", func(t *testing.T) {
		l := kv.New[string]()
		l.Set("a", "1")
		entries := l.Entries()
		entries[0].Value = "mutated"
		assert.Equal(t, "1", l.Entries()[0].Value)
	})
}
