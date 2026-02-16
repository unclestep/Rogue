package algorithm

type node[T any] struct {
	pos    T
	cost   float64 // Cost from start to this node
	rank   float64 // Actual cost + estimated cost
	parent *node[T]
	index  int
	closed bool // Skip this node if it's true
}

// PQueue a queue of nodes
type PQueue[T any] []*node[T]

// Len - wrap on len function
func (pq PQueue[T]) Len() int {
	return len(pq)
}

// Less - a given compare result between two items
func (pq PQueue[T]) Less(i, j int) bool {
	return pq[i].rank < pq[j].rank
}

// Swap - swaps two items in the queue
func (pq PQueue[T]) Swap(i, j int) {
	pq[i], pq[j] = pq[j], pq[i]
	pq[i].index = i
	pq[j].index = j
}

// Push - adds item to the queue
func (pq *PQueue[T]) Push(x any) {
	n := len(*pq)
	item := x.(*node[T])
	item.index = n
	*pq = append(*pq, item)
}

// Pop - removes and return last item from the queue
func (pq *PQueue[T]) Pop() any {
	old := *pq
	n := old.Len()
	item := old[n-1]
	item.index = -1
	*pq = old[0 : n-1]
	return item
}
