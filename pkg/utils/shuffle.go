package utils

import (
	"math/rand"
)

// shuffle - shuffles the slice
func Shuffle[T any](r *rand.Rand, s []T) {
	r.Shuffle(len(s), func(i, j int) {
		s[i], s[j] = s[j], s[i]
	})
}
