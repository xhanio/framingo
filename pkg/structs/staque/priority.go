package staque

import (
	"context"
	"sync"

	"github.com/google/btree"

	"github.com/xhanio/errors"
	"github.com/xhanio/framingo/pkg/utils/log"
)

type priority[T PriorityItem] struct {
	log log.Logger

	sync.RWMutex
	items  map[string]T
	lf     btree.LessFunc[T]
	tree   *btree.BTreeG[T]
	pushed *sync.Cond // broadcast as elements are pushed, for those waiting on an empty queue
}

// New initializes an empty priority queue.
func NewPriority[T PriorityItem](opts ...Option[T]) Priority[T] {
	p := &priority[T]{
		items: make(map[string]T),
	}
	p.applyOptions(opts...)
	p.setDefaults()
	p.pushed = sync.NewCond(&p.RWMutex)
	p.tree = btree.NewG(2, p.lf)
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
		p.pushed.Broadcast()
	}
}

func (p *priority[T]) Update(items ...T) error {
	p.Lock()
	defer p.Unlock()
	for _, item := range items {
		if _, ok := p.items[item.Key()]; !ok {
			return errors.NotFound.Newf("failed to update item: key %s not found", item.Key())
		}
	}
	for _, item := range items {
		i := p.items[item.Key()]
		// out of the tree while its priority changes, which is where the tree finds it
		if _, found := p.tree.Delete(i); !found {
			panic(errors.Newf("inconsistent queue: item %s is not in the tree", item.Key()))
		}
		i.SetPriority(item.Priority())
		p.tree.ReplaceOrInsert(i)
	}
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
		return *new(T), errors.Newf("failed to pop element: the queue is empty")
	}
	delete(p.items, item.Key())
	return item, nil
}

func (p *priority[T]) MustPop() T {
	element, err := p.Pop()
	if err != nil {
		return *new(T)
	}
	return element
}

func (p *priority[T]) Shift() (T, error) {
	p.Lock()
	defer p.Unlock()
	item, ok := p.tree.DeleteMin()
	if !ok {
		return *new(T), errors.Newf("failed to pop element: the queue is empty")
	}
	delete(p.items, item.Key())
	return item, nil
}

func (p *priority[T]) MustShift() T {
	element, err := p.Shift()
	if err != nil {
		return *new(T)
	}
	return element
}

// WaitPop is Pop that waits while the queue is empty, until ctx ends.
func (p *priority[T]) WaitPop(ctx context.Context) (T, error) {
	return p.wait(ctx, p.tree.DeleteMax)
}

// WaitShift is Shift that waits while the queue is empty, until ctx ends.
func (p *priority[T]) WaitShift(ctx context.Context) (T, error) {
	return p.wait(ctx, p.tree.DeleteMin)
}

// wait takes an element with take once there is one. Once ctx ends it returns ctx's error and
// takes nothing.
func (p *priority[T]) wait(ctx context.Context, take func() (T, bool)) (T, error) {
	stop := context.AfterFunc(ctx, func() {
		p.Lock()
		defer p.Unlock()
		p.pushed.Broadcast()
	})
	defer stop()
	p.Lock()
	defer p.Unlock()
	for {
		if err := ctx.Err(); err != nil {
			return *new(T), err
		}
		if item, ok := take(); ok {
			delete(p.items, item.Key())
			return item, nil
		}
		p.pushed.Wait()
	}
}

// !! the order of items is not guaranteed
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
