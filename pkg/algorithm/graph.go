package algorithm

// Graph - graph interface for A* algroithm
type Graph[T comparable] interface {
	GetNeighbors(n T) []T
	CalcHeuristic(n1, n2 T) float64
}
