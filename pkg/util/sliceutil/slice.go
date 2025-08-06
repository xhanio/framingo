package sliceutil

// This function checks whether the first element is present in the subsequent elements
// It has a time complexity of O(n)
func In[T comparable](element T, elements ...T) bool {
	for _, e := range elements {
		if e == element {
			return true
		}
	}
	return false
}

// This function returns the first non-default element in the elements
// It has a time complexity of O(n)
func First[T comparable](elements ...T) T {
	var undefined T
	for _, e := range elements {
		if e != undefined {
			return e
		}
	}
	return undefined
}

// This function returns the last non-default element in the elements
// It has a time complexity of O(n)
func Last[T comparable](elements ...T) T {
	var undefined T
	for i := len(elements) - 1; i >= 0; i-- {
		e := elements[i]
		if e != undefined {
			return e
		}
	}
	return undefined
}

// TODO: FirstIndex & LastIndex

func IsDiff[T comparable](a []T, b []T) bool {
	if len(a) != len(b) {
		return true
	}
	for i := range a {
		if a[i] != b[i] {
			return true
		}
	}
	return false
}

func Deduplicate[T comparable](elements ...T) []T {
	var result []T
	unique := make(map[T]any)
	for _, elem := range elements {
		if _, ok := unique[elem]; !ok {
			unique[elem] = true
			result = append(result, elem)
		}
	}
	return result
}

func Remove[T comparable](target T, elements ...T) []T {
	var result []T
	for _, elem := range elements {
		if target != elem {
			result = append(result, elem)
		}
	}
	return result
}

func Copy[T any](source []T) []T {
	target := make([]T, len(source))
	copy(target, source)
	return target
}

func Diff[T comparable](a []T, b []T) ([]T, []T) {
	am := make(map[T]int)
	bm := make(map[T]int)
	for _, item := range a {
		am[item]++
	}
	for _, item := range b {
		bm[item]++
	}
	var toCreate, toDelete []T
	for item, ac := range am {
		bc := bm[item]
		if ac > bc {
			for i := 0; i < ac-bc; i++ {
				toCreate = append(toCreate, item)
			}
		}
	}
	for item, bc := range bm {
		ac := am[item]
		if bc > ac {
			for i := 0; i < bc-ac; i++ {
				toDelete = append(toDelete, item)
			}
		}
	}
	return toCreate, toDelete
}
