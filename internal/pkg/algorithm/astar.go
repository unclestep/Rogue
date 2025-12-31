package algorithm

import (
	"container/heap"
	"github.com/Nikolay-Yakunin/gouge/internal/pkg/geometry"
	"math"
)

type Graph interface {
	GetCost(p geometry.Point) float64
	GetNeighbors(p geometry.Point, digging bool) []geometry.Point
}

func calcHeuristic(p1, p2 geometry.Point) float64 {
	x1, y1 := p1.X, p1.Y
	x2, y2 := p2.X, p2.Y
	return math.Abs(float64(x2-x1)) + math.Abs(float64(y2-y1))
}

func reconstructPath(endpoint *node) []geometry.Point {
	path := make([]geometry.Point, 0)
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

// Finds path between two points
// digging=true allows to pass through the walls
// digging=false can be useful for following
func FindPath(g Graph, initial, target geometry.Point, digging bool) []geometry.Point {
	pq := make(PQueue, 0)
	initialNode := &node{
		pos:  initial,
		rank: calcHeuristic(initial, target),
	}
	heap.Push(&pq, initialNode)

	nodeMap := make(map[geometry.Point]*node)
	nodeMap[initial] = initialNode

	for len(pq) > 0 {
		cur := heap.Pop(&pq).(*node)
		nodeMap[cur.pos].closed = true

		if cur.pos == target {
			return reconstructPath(cur)
		}

		for _, neighbor := range g.GetNeighbors(cur.pos, digging) {
			n, visited := nodeMap[neighbor]

			if visited && n.closed {
				continue
			}

			newCost := cur.cost + g.GetCost(neighbor)
			newRank := newCost + calcHeuristic(neighbor, target)

			if !visited {
				newNode := &node{
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
