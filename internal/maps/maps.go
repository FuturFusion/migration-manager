package maps

func GetOrDefault[K comparable, V any](lookup map[K]any, key K, defaultValue V) V {
	anyValue, ok := lookup[key]
	if !ok {
		return defaultValue
	}

	value, ok := anyValue.(V)
	if !ok {
		return defaultValue
	}

	return value
}
