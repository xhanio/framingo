package paramutil_test

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/xhanio/framingo/pkg/utils/paramutil"
)

func TestEntries_Set(t *testing.T) {
	t.Run("appends new keys in the order they are set", func(t *testing.T) {
		l := paramutil.NewEntries[string]()
		l.Set("a", "1")
		l.Set("b", "2")
		l.Set("c", "3")
		assert.Equal(t, []paramutil.Param[string]{
			{Key: "a", Value: "1", Keyed: true},
			{Key: "b", Value: "2", Keyed: true},
			{Key: "c", Value: "3", Keyed: true},
		}, l.List())
	})

	t.Run("overwrites a known key in place", func(t *testing.T) {
		l := paramutil.NewEntries[string]()
		l.Set("a", "1")
		l.Set("b", "2")
		l.Set("a", "3")
		assert.Equal(t, []paramutil.Param[string]{
			{Key: "a", Value: "3", Keyed: true},
			{Key: "b", Value: "2", Keyed: true},
		}, l.List())
	})

	t.Run("treats the empty string as a key", func(t *testing.T) {
		l := paramutil.NewEntries[string]()
		l.Set("", "1")
		l.Set("", "2")
		assert.Equal(t, []paramutil.Param[string]{{Value: "2", Keyed: true}}, l.List())
	})
}

func TestEntries_AddRaw(t *testing.T) {
	t.Run("keeps unkeyed entries in the position they were added", func(t *testing.T) {
		l := paramutil.NewEntries[string]()
		l.Set("a", "1")
		l.AddRaw("x")
		l.Set("b", "2")
		assert.Equal(t, []paramutil.Param[string]{
			{Key: "a", Value: "1", Keyed: true},
			{Raw: "x"},
			{Key: "b", Value: "2", Keyed: true},
		}, l.List())
	})

	t.Run("never merges unkeyed entries with each other", func(t *testing.T) {
		l := paramutil.NewEntries[string]()
		l.AddRaw("x")
		l.AddRaw("x")
		assert.Len(t, l.List(), 2)
	})

	t.Run("does not let an unkeyed entry answer a Get", func(t *testing.T) {
		l := paramutil.NewEntries[string]()
		l.AddRaw("a")
		_, ok := l.Get("a")
		assert.False(t, ok)
	})
}

func TestEntries_Get(t *testing.T) {
	t.Run("returns the last value set", func(t *testing.T) {
		l := paramutil.NewEntries[string]()
		l.Set("a", "1")
		l.Set("a", "2")
		got, ok := l.Get("a")
		assert.True(t, ok)
		assert.Equal(t, "2", got)
	})

	t.Run("reports a key that was never set", func(t *testing.T) {
		l := paramutil.NewEntries[string]()
		got, ok := l.Get("a")
		assert.False(t, ok)
		assert.Empty(t, got)
	})

	t.Run("hands back a stored pointer so callers can mutate through it", func(t *testing.T) {
		l := paramutil.NewEntries[*[]string]()
		values := []string{"1"}
		l.Set("a", &values)
		got, ok := l.Get("a")
		assert.True(t, ok)
		*got = append(*got, "2")
		assert.Equal(t, []string{"1", "2"}, *l.List()[0].Value)
	})
}

func TestEntries_Len(t *testing.T) {
	l := paramutil.NewEntries[string]()
	assert.Equal(t, 0, l.Len())
	l.Set("a", "1")
	l.AddRaw("x")
	l.Set("a", "2")
	assert.Equal(t, 2, l.Len())
}

func TestEntries_List(t *testing.T) {
	t.Run("returns nothing for an Entries never written to", func(t *testing.T) {
		l := paramutil.NewEntries[string]()
		assert.Empty(t, l.List())
	})

	t.Run("does not alias the entries", func(t *testing.T) {
		l := paramutil.NewEntries[string]()
		l.Set("a", "1")
		entries := l.List()
		entries[0].Value = "mutated"
		assert.Equal(t, "1", l.List()[0].Value)
	})
}
