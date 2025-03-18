package maps

// TODO(juf): Debattable
func UnionInPlace[K comparable, V any](a, b map[K]V) map[K]V {
	copyFrom(a, b)
	return b
}

// TODO(juf): Debattable
func Union[K comparable, V any](a, b map[K]V) map[K]V {
	union := make(map[K]V)
	copyFrom(a, union)
	copyFrom(b, union)
	return union
}

func copyFrom[K comparable, V any](from, into map[K]V) {
	for k, v := range from {
		into[k] = v
	}
}
