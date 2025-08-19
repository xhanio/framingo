package trie

type Node[T any] interface {
	Value() T
}
