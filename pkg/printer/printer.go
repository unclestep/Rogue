package printer

import (
	"fmt"
	"strconv"
)

func PrintMatrix[T ~int](m [][]T) {
	maxVal := m[0][0]
	for i := range m {
		for j := range m[i] {
			if m[i][j] > maxVal {
				maxVal = m[i][j]
			}
		}
	}

	maxValLen := len(strconv.Itoa(int(maxVal)))

	for i := range m {
		for j := range m[i] {
			fmt.Printf("%0*v ", maxValLen, m[i][j])
		}
		fmt.Println()
	}
}
