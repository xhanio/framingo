package queue

import (
	"encoding/json"
	"sync"

	"xhanio/framingo/pkg/utils/errors"
)

type simple[T any] struct {
	sync.RWMutex
	data    []T
	maxSize int
}

// create a new simple.
// max size <= 0 means no size limit.
func New[T any](maxSize int) Queue[T] {
	return newQueue[T](maxSize)
}

// test purpose
func newQueue[T any](maxSize int) *simple[T] {
	if maxSize < 0 {
		maxSize = 0
	}
	return &simple[T]{
		data:    make([]T, 0),
		maxSize: maxSize,
	}
}

func (s *simple[T]) MustGet(i int) T {
	s.RLock()
	defer s.RUnlock()
	if i >= 0 && i < len(s.data) {
		return s.data[i]
	}
	return *new(T)
}

func (s *simple[T]) Reset() {
	s.Lock()
	defer s.Unlock()
	s.data = make([]T, 0)
}

func (s *simple[T]) Length() int {
	s.RLock()
	defer s.RUnlock()
	return len(s.data)
}

func (s *simple[T]) IsEmpty() bool {
	s.RLock()
	defer s.RUnlock()
	return len(s.data) == 0
}

func (s *simple[T]) Push(element ...T) {
	s.Lock()
	defer s.Unlock()
	s.data = append(s.data, element...)
	// adjust by max size
	if s.maxSize > 0 && len(s.data) > s.maxSize {
		s.data = s.data[len(s.data)-s.maxSize:]
	}
}

func (s *simple[T]) Pop() (T, error) {
	s.Lock()
	defer s.Unlock()
	if len(s.data) == 0 {
		return *new(T), errors.Newf("failed to pop element: the simple is empty")
	} else {
		last := len(s.data) - 1
		element := (s.data)[last]
		s.data = (s.data)[:last]
		return element, nil
	}
}

func (s *simple[T]) MustPop() T {
	element, err := s.Pop()
	if err != nil {
		return *new(T)
	}
	return element
}

func (s *simple[T]) PopN(n int) ([]T, error) {
	s.Lock()
	defer s.Unlock()
	if len(s.data) == 0 {
		return nil, errors.Newf("failed to pop elements: simple is empty")
	} else {
		l := len(s.data)
		last := l - n
		if last < 0 {
			return nil, errors.Newf("failed to pop elements: required %d, found only %d in simple", n, l)
		}
		elements := make([]T, n)
		copy(elements, (s.data)[last:])
		s.data = (s.data)[:last]
		return elements, nil
	}
}

func (s *simple[T]) Shift() (T, error) {
	s.Lock()
	defer s.Unlock()
	if len(s.data) == 0 {
		return *new(T), errors.Newf("failed to shift element: simple is empty")
	} else {
		element := s.data[0]
		s.data = s.data[1:]
		return element, nil
	}
}

func (s *simple[T]) MustShift() T {
	element, err := s.Shift()
	if err != nil {
		return *new(T)
	}
	return element
}

func (s *simple[T]) ShiftN(n int) ([]T, error) {
	s.Lock()
	defer s.Unlock()
	if len(s.data) == 0 {
		return nil, errors.Newf("failed to pop elements: simple is empty")
	} else {
		l := len(s.data)
		if n > l {
			return nil, errors.Newf("failed to pop elements: required %d, found only %d in simple", n, l)
		}
		elements := make([]T, n)
		copy(elements, s.data[:n])
		s.data = s.data[n:]
		return elements, nil
	}
}

func toQueue[T any](data any) (*simple[T], error) {
	if data == nil {
		return &simple[T]{data: make([]T, 0)}, nil
	}
	switch d := data.(type) {
	case simple[T]:
		return &d, nil
	case []T:
		return &simple[T]{data: d}, nil
	case []any:
		r := simple[T]{data: make([]T, 0)}
		for _, e := range d {
			elem, err := convert[T](e)
			if err == nil {
				r.data = append(r.data, elem)
			} else {
				r.data = append(r.data, *new(T))
			}
		}
		return &r, nil
	default:
		return nil, errors.Newf("failed to convert into slice: unknown type %T", d)
	}
}

func convert[T any](data any) (T, error) {
	if r, ok := data.(T); ok {
		return r, nil
	}
	b, err := json.Marshal(&data)
	if err != nil {
		return *new(T), errors.Wrapf(err, "")
	}
	var r T
	err = json.Unmarshal(b, &r)
	if err != nil {
		return *new(T), errors.Wrapf(err, "")
	}
	return r, nil
}
