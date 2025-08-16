package graph

import (
	"github.com/xhanio/framingo/pkg/structs/staque"
	"github.com/xhanio/framingo/pkg/types/common"
	"github.com/xhanio/framingo/pkg/utils/errors"
	"github.com/xhanio/framingo/pkg/utils/maputil"
)

type graph[T common.Named] struct {
	added   maputil.Set[string]
	nodes   []T
	edges   map[string][]T
	visited maputil.Set[string]
	exists  maputil.Set[string]
}

func newGraph[T common.Named]() *graph[T] {
	return &graph[T]{
		added:   make(maputil.Set[string]),
		nodes:   make([]T, 0),
		edges:   make(map[string][]T),
		visited: make(maputil.Set[string]),
		exists:  make(maputil.Set[string]),
	}
}

func New[T common.Named]() Graph[T] {
	return newGraph[T]()
}

func (g *graph[T]) Add(node T, dependencies ...T) {
	g.add(node)
	for _, dep := range dependencies {
		g.add(dep)
		g.edges[dep.Name()] = append(g.edges[dep.Name()], node)
	}
}

func (g *graph[T]) add(node T) {
	if !g.added.Has(node.Name()) {
		g.added.Add(node.Name())
		g.nodes = append(g.nodes, node)
	}
}

func (g *graph[T]) dfs(v T, stack staque.Simple[T]) bool {
	name := v.Name()
	g.visited.Add(name)
	g.exists.Add(name)
	for _, neighbor := range g.edges[name] {
		neighborName := neighbor.Name()
		if !g.visited.Has(neighborName) {
			if g.dfs(neighbor, stack) {
				return true
			}
		} else if g.exists.Has(neighborName) {
			return true
		}
	}
	g.exists.Remove(name)
	stack.Push(v)
	return false
}

func (g *graph[T]) TopoSort() error {
	g.visited = make(maputil.Set[string], len(g.nodes))
	g.exists = make(maputil.Set[string], len(g.nodes))

	stack := staque.NewSimple[T](len(g.nodes))
	for _, node := range g.nodes {
		if !g.visited.Has(node.Name()) {
			if g.dfs(node, stack) {
				return errors.Conflict.Newf("graph contains a cycle")
			}
		}
	}

	var result []T
	for !stack.IsEmpty() {
		result = append(result, stack.MustPop())
	}
	g.nodes = result
	return nil
}

func (g *graph[T]) Nodes() []T {
	return g.nodes
}

func (g *graph[T]) Count() int {
	return len(g.nodes)
}
