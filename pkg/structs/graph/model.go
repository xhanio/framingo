package graph

import "github.com/xhanio/framingo/pkg/types/common"

type Graph[T common.Named] interface {
	Add(node T, dependencies ...T)
	TopoSort() error
	Nodes() []T
	Count() int
}
