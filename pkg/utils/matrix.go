package utils

// CreateMatrix - creates a matrix with given rows and cols
func CreateMatrix[T any](rows, cols int) [][]T {
	matrix := make([][]T, rows)
	for i := range matrix {
		matrix[i] = make([]T, cols)
	}
	return matrix
}

// ClearMatrix - clears the matrix
func ClearMatrix[T any](matrix [][]T) {
	for i := range matrix {
		clear(matrix[i])
	}
}

// FillMatrix - fills matrix using given value
func FillMatrix[T any](matrix [][]T, val T) {
	for i := range len(matrix) {
		for j := range len(matrix[i]) {
			matrix[i][j] = val
		}
	}
}

func CloneMatrix[T any](m [][]T) [][]T {
	clone := make([][]T, len(m))
	for row := range m {
		clone[row] = make([]T, len(m))
		copy(clone[row], m[row])
	}
	return clone
}
