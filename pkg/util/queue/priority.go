package queue

import (
	"sync"

	"github.com/google/btree"

	"xhanio/framingo/pkg/util/errors"
	"xhanio/framingo/pkg/util/log"
)

type priority[T PriorityItem] struct {
	log log.Logger

	sync.RWMutex
	items    map[string]T
	lf       btree.LessFunc[T]
	tree     *btree.BTreeG[T]
	empty    *sync.Cond
	blocking bool
}

// New initializes an empty priority queue.
func NewPriority[T PriorityItem](opts ...Option[T]) PriorityQueue[T] {
	p := &priority[T]{
		items: make(map[string]T),
		lf:    DefaultLessFunc[T],
	}
	for _, opt := range opts {
		opt(p)
	}
	p.empty = sync.NewCond(&p.RWMutex)
	p.tree = btree.NewG(2, p.lf)
	if p.log == nil {
		p.log = log.Default
	}
	return p
}

func (p *priority[T]) IsEmpty() bool {
	p.RLock()
	defer p.RUnlock()
	return len(p.items) == 0
}

func (p *priority[T]) Length() int {
	p.RLock()
	defer p.RUnlock()
	if len(p.items) != p.tree.Len() {
		panic(errors.Newf("inconsistent queue length: items %d tree %d", len(p.items), p.tree.Len()))
	}
	return len(p.items)
}

func (p *priority[T]) Push(items ...T) {
	p.Lock()
	defer p.Unlock()
	for _, item := range items {
		if _, ok := p.items[item.Key()]; !ok {
			p.items[item.Key()] = item
			p.tree.ReplaceOrInsert(item)
		}
	}
	if len(p.items) > 0 {
		p.empty.Signal()
	}
}

func (p *priority[T]) Update(item T) error {
	p.Lock()
	defer p.Unlock()
	i, ok := p.items[item.Key()]
	if !ok {
		return errors.NotFound.Newf("failed to update item: key %s not found", item.Key())
	}
	i.SetPriority(item.GetPriority())
	p.tree.ReplaceOrInsert(i)
	return nil
}

func (p *priority[T]) Remove(item T) (T, bool) {
	p.Lock()
	defer p.Unlock()
	i, ok := p.items[item.Key()]
	if !ok {
		return *new(T), false
	}
	deleted, found := p.tree.Delete(i)
	if found {
		delete(p.items, item.Key())
	}
	if ok != found {
		panic(errors.Newf("inconsistent queue length: items %d tree %d", len(p.items), p.tree.Len()))
	}
	return deleted, found
}

func (p *priority[T]) Pop() (T, error) {
	p.Lock()
	defer p.Unlock()
	item, ok := p.tree.DeleteMax()
	if !ok {
		if p.blocking {
			p.empty.Wait()
		}
		return *new(T), errors.Newf("failed to pop element: the queue is empty")
	}
	delete(p.items, item.Key())
	return item, nil
}

func (p *priority[T]) Items() []T {
	p.RLock()
	defer p.RUnlock()
	var result []T
	for _, item := range p.items {
		result = append(result, item)
	}
	return result
}

func (p *priority[T]) Reset() {
	p.Lock()
	defer p.Unlock()
	p.tree.Clear(false)
	p.items = make(map[string]T)
}
