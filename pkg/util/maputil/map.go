package maputil

func CopyKeys[K comparable, V any](from map[K]V) map[K]V {
	result := make(map[K]V)
	for k := range from {
		result[k] = from[k]
	}
	return result
}

func DiffKeys[K comparable, V any](from map[K]V, to map[K]V) (map[K]V, map[K]V) {
	toCreate := make(map[K]V)
	toDelete := make(map[K]V)
	for k := range from {
		toDelete[k] = from[k]
	}
	for k := range to {
		if _, ok := from[k]; !ok {
			toCreate[k] = to[k]
		} else {
			delete(toDelete, k)
		}
	}
	return toCreate, toDelete
}

func In[K comparable, V any](m map[K]V, keys ...K) bool {
	for _, k := range keys {
		if _, ok := m[k]; !ok {
			return false
		}
	}
	return true
}

func Keys[K comparable, V any](m map[K]V) []K {
	var result []K
	for k := range m {
		result = append(result, k)
	}
	return result
}

func Values[K comparable, V any](m map[K]V) []V {
	var result []V
	for k := range m {
		result = append(result, m[k])
	}
	return result
}
