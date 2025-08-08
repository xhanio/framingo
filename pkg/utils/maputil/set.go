package maputil

type Set[E comparable] map[E]any

func (s Set[E]) Add(element E) {
	s[element] = nil
}

func (s Set[E]) Remove(element E) {
	delete(s, element)
}

func (s Set[E]) Has(element E) bool {
	_, ok := s[element]
	return ok
}
