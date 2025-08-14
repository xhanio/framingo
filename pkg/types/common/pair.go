package common

import (
	"fmt"
	"strings"

	"github.com/xhanio/framingo/pkg/utils/strutil"
)

var _ Pair[string, string] = (*pair[string, string])(nil)

type Pair[K comparable, V any] interface {
	SetKey(key K)
	SetValue(value V)
	GetKey() K
	GetValue() V
	String() string
}

type pair[K comparable, V any] struct {
	key   K
	value V
}

func NewPair[K comparable, V any](key K, value V) Pair[K, V] {
	return &pair[K, V]{
		key:   key,
		value: value,
	}
}

func (p *pair[K, V]) SetKey(key K) {
	if p == nil {
		p = &pair[K, V]{}
	}
	p.key = key
}

func (p *pair[K, V]) SetValue(value V) {
	if p == nil {
		p = &pair[K, V]{}
	}
	p.value = value
}

func (p *pair[K, V]) GetKey() K {
	if p == nil {
		return *new(K)
	}
	return p.key
}

func (p *pair[K, V]) GetValue() V {
	if p == nil {
		return *new(V)
	}
	return p.value
}

func (p *pair[K, V]) String() string {
	if p == nil {
		return ""
	}
	return fmt.Sprintf("%v=%v", p.key, p.value)
}

type Pairs []Pair[string, string]

func NewPairs[T Pair[string, string]](src []T) Pairs {
	var result []Pair[string, string]
	for _, elem := range src {
		result = append(result, elem)
	}
	return result
}

func (p Pairs) String() string {
	return strutil.Join(",", p...)
}

func (p Pairs) Map() map[string]string {
	result := make(map[string]string)
	for _, pair := range p {
		result[pair.GetKey()] = pair.GetValue()
	}
	return result
}

func ParsePair(s string) (Pair[string, string], bool) {
	kv := strings.SplitN(s, "=", 2)
	if len(kv) != 2 {
		return nil, false
	}
	return NewPair(kv[0], kv[1]), true
}

func ParsePairs(s string) (Pairs, bool) {
	var result Pairs
	every := true
	for _, sp := range strings.Split(s, ",") {
		if p, ok := ParsePair(sp); ok {
			result = append(result, p)
		} else {
			every = false
		}
	}
	return result, every
}

func ParsePairsMap(m map[string]string) Pairs {
	var result Pairs
	for k, v := range m {
		result = append(result, NewPair(k, v))
	}
	return result
}
