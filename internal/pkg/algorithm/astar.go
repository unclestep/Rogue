// Package algorithm implements the internals algorithms and data structures, like:
// - A*;
// - PQueue;
package algorithm

import (
	"container/heap"
	"math"
)

// Graph interface of map for A* algorithm
// interface on consumer side
type Graph[T comparable] interface {
	GetNeighbors(n T) []T
	CalcHeuristic(n1, n2 T) float64
}

// reconstructPath builds path from start to end node
func reconstructPath[T comparable](endpoint *node[T]) []T {
	path := make([]T, 0)
	cur := endpoint

	for cur != nil {
		path = append(path, cur.pos)
		cur = cur.parent
	}

	for i, j := 0, len(path)-1; i < j; i, j = i+1, j-1 {
		path[i], path[j] = path[j], path[i]
	}

	return path
}

// FindPath - Finds path between two points
// A* implementation
func FindPath[T comparable](g Graph[T], initial, target T, getCost func(n T) float64) []T {
	if math.IsInf(getCost(initial), 1) || math.IsInf(getCost(target), 1) {
		return nil
	}

	pq := make(PQueue[T], 0)
	initialNode := &node[T]{
		pos:  initial,
		rank: g.CalcHeuristic(initial, target),
	}
	heap.Push(&pq, initialNode)

	nodeMap := make(map[T]*node[T])
	nodeMap[initial] = initialNode

	for len(pq) > 0 {
		cur := heap.Pop(&pq).(*node[T])
		nodeMap[cur.pos].closed = true

		if cur.pos == target {
			return reconstructPath(cur)
		}

		for _, neighbor := range g.GetNeighbors(cur.pos) {
			neighborCost := getCost(neighbor)
			if math.IsInf(neighborCost, 1) {
				continue
			}

			n, visited := nodeMap[neighbor]
			if visited && n.closed {
				continue
			}

			newCost := cur.cost + neighborCost
			if math.IsInf(newCost, 1) {
				continue
			}

			newRank := newCost + g.CalcHeuristic(neighbor, target)

			if !visited {
				newNode := &node[T]{
					pos:    neighbor,
					cost:   newCost,
					rank:   newRank,
					parent: cur,
				}
				nodeMap[neighbor] = newNode
				heap.Push(&pq, newNode)
			} else if newCost < n.cost {
				n.cost = newCost
				n.parent = cur
				n.rank = newRank
				heap.Fix(&pq, n.index)
			}
		}
	}
	return nil
}
