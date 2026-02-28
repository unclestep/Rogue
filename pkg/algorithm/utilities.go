package algorithm

// Remove - Swap and delete method (order is not preserved)
func Remove[T any](s []T, i int) []T {
	var zero T
	last := len(s) - 1
	s[i], s[last] = s[last], zero
	return s[:last]
}

// RemoveOrderly - removes item with given index and preserve the order
func RemoveOrderly[T any](s []T, i int) []T {
	return append(s[:i], s[i+1:]...)
}
