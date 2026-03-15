package conv

type KV[K comparable, V any] struct {
	Key K
	Val V
}

func MapToKV[K comparable, V any](m map[K]V) []KV[K, V] {
	kv := make([]KV[K, V], 0, len(m))

	for key, val := range m {
		kv = append(kv, KV[K, V]{Key: key, Val: val})
	}

	return kv
}

func MapKeysToSlice[K comparable, V any](m map[K]V) []K {
	s := make([]K, 0, len(m))

	for key := range m {
		s = append(s, key)
	}

	return s
}

func MapValsToSlice[K comparable, V any](m map[K]V) []V {
	s := make([]V, 0, len(m))

	for _, val := range m {
		s = append(s, val)
	}

	return s
}
