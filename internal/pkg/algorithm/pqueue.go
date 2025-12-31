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

type PQueue []*node

func (pq PQueue) Len() int {
	return len(pq)
}

// Pop method will give an item with the lowest priority
func (pq PQueue) Less(i, j int) bool {
	return pq[i].rank < pq[j].rank
}

func (pq PQueue) Swap(i, j int) {
	pq[i], pq[j] = pq[j], pq[i]
	pq[i].index = i
	pq[j].index = j
}

func (pq *PQueue) Push(x any) {
	n := len(*pq)
	item := x.(*node)
	item.index = n
	*pq = append(*pq, item)
}

func (pq *PQueue) Pop() any {
	old := *pq
	n := len(old)
	item := old[n-1]
	item.index = -1
	*pq = old[0 : n-1]
	return item
}
