package algorithm

import (
	"github.com/Nikolay-Yakunin/gouge/internal/pkg/geometry"
)

type node struct {
	pos    geometry.Point
	cost   float64 // Cost from start to this node
	rank   float64 // Actual cost + estimated cost
	parent *node
	index  int
	closed bool // Skip this node if it's true
}

// PQueue a queue of nodes
type PQueue []*node

// Len - wrap on len function
func (pq PQueue) Len() int {
	return len(pq)
}

// Less - a given compare result between two items
func (pq PQueue) Less(i, j int) bool {
	return pq[i].rank < pq[j].rank
}

// Swap - swaps two items in the queue
func (pq PQueue) Swap(i, j int) {
	pq[i], pq[j] = pq[j], pq[i]
	pq[i].index = i
	pq[j].index = j
}

// Push - adds item to the queue
func (pq *PQueue) Push(x any) {
	n := len(*pq)
	item := x.(*node)
	item.index = n
	*pq = append(*pq, item)
}

// Pop - removes and return last item from the queue
func (pq *PQueue) Pop() any {
	old := *pq
	n := old.Len()
	item := old[n-1]
	item.index = -1
	*pq = old[0 : n-1]
	return item
}
