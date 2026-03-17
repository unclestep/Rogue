package service

import (
	"log"
	"math/rand"
	"slices"

	"github.com/unclestep/Rogue/internal/domain/model"
	"github.com/unclestep/Rogue/pkg/algorithm"
	"github.com/unclestep/Rogue/pkg/geometry"
	"github.com/unclestep/Rogue/pkg/utils"
)

// DoorLocker - service which locks doors on the map
type DoorLocker struct{}

func NewDoorLocker() *DoorLocker {
	return &DoorLocker{}
}

const (
	MaxKeysForRoom = 1
)

// LockDoors - generates doorsCount doors and keysCount keys.
// If doorsCount > keysCount, some keys can open more than one door;
// if keysCount > doorsCount, keysCount = doorsCount;
// if doorsCount is greater than total number of doors, algorithm will block all found bridges on the map;
// if doorsCount < 0 || keysCount < 0, nil will be returned.
func (d *DoorLocker) LockDoors(ctx *model.SessionContext, doorCount, keyCount int) {
	if doorCount <= 0 || keyCount <= 0 {
		log.Printf("[INFO] Got: doorCount=%d, keyCount=%d, expected: doorCount>0 and keyCount>0", doorCount, keyCount)
		return
	}

	m := ctx.Playthrough.Map
	if m == nil {
		log.Printf("[ERROR] Map topology is not generated\n")
		return
	}

	nm := createNavMap(ctx.Playthrough)
	entranceRoom := nm.findEntrance(m.EntranceRoomId)
	usedCells := make(map[NavNode]int)

	validKeysCount := min(doorCount, keyCount)
	validDoorsCount := min(validKeysCount, len(m.GetDoors()))
	doorsForKey := getDoorsForKey(validDoorsCount, validKeysCount, ctx.Rng())
	curKeyhole := model.Keyhole(1)

	for k := range validKeysCount {
		topology := nm.analyzeTopology(entranceRoom, usedCells)

		numDoors := doorsForKey[k]          // Get number of doors to be blocked by current key
		keysRemaining := validKeysCount - k // Keys left

		curLockedEdges := make([]*Edge, 0) // Slice of locked edges by current key

		// Start blocking the doors
		for range numDoors {
			availableEdges := nm.getAvailableEdges(topology, entranceRoom)
			utils.Shuffle(ctx.Rng(), availableEdges)

			totalCapacity := topology.SubtreeCapacity[entranceRoom]
			totalDoors := topology.SubtreeDoors[entranceRoom]

			if len(availableEdges) == 0 {
				break
			}

			var selectedEdge *Edge
			maxAvailableDoors := -1

			// Find convenient room in the path
			for _, edge := range availableEdges {
				// It is integral to correctly define the node which will be locked
				// Unavailable node is always the one that's further from the center
				node := topology.determineFurtherNode(edge)

				cutDoors := topology.SubtreeDoors[node] // Number of doors that we lose after locking this edge
				availableDoors := totalDoors - cutDoors // Number of doors that will be still available

				cutCells := topology.SubtreeCapacity[node] // Number of free cells (to place the key) we lose after locking this edge
				availableCells := totalCapacity - cutCells // Number of cells that will be still available

				// This condition checks if there is enough of remain doors and cells
				// availableCells convince to cut edges with enough remain cells
				if availableDoors >= (keysRemaining-1) && availableCells >= keysRemaining {
					selectedEdge = edge
					break
				}

				// If we can't satisfy some of the conditions (or all of them), take the edge that will be the closest one to satisfy them
				if availableCells >= keysRemaining && availableDoors > maxAvailableDoors {
					maxAvailableDoors = availableDoors
					selectedEdge = edge
				}
			}

			// If we couldn't find the edge that satisfies all the conditions, take the optimal one
			if selectedEdge == nil {
				break
			}

			curLockedEdges = append(curLockedEdges, selectedEdge)
			nm.deleteEdge(selectedEdge)
			topology = nm.analyzeTopology(entranceRoom, usedCells)
		}

		// If we couldn't find any edge to lock, stop the locking algorithm:
		// we won't be able to find convenient edges for other doors and keys either
		if len(curLockedEdges) == 0 {
			break
		}

		// Define the rooms where we can the spawn the door key
		availableRooms := make([]*RoomWrp, 0, len(nm.rooms))
		for _, r := range nm.rooms {
			// Check if the room is in locked subtree
			if _, reachable := topology.EntryTime[r]; !reachable {
				continue
			}

			// If there is no space or consumed all the available cells
			if r == entranceRoom || usedCells[r] >= MaxKeysForRoom || len(r.r.GetEmptyItemPoints()) <= 0 {
				continue
			}

			if keysRemaining == 1 {
				availableRooms = append(availableRooms, r)
				continue
			}

			// We are looking at how the spawn will affect subsequent locks
			usedCells[r]++
			testTopology := nm.analyzeTopology(entranceRoom, usedCells)
			totalCapacity := testTopology.SubtreeCapacity[entranceRoom]

			for bridge := range testTopology.Bridges {
				node := testTopology.determineFurtherNode(bridge)
				cutCells := testTopology.SubtreeCapacity[node]
				availableCells := totalCapacity - cutCells

				// If we have found at least one bridge that can be safely blocked, then the room is safe.
				if availableCells >= (keysRemaining - 1) {
					availableRooms = append(availableRooms, r)
					break
				}
			}
			usedCells[r]--
		}

		// No reason to continue iterating if there is no more available rooms; won't be able to find for other keys too
		if len(availableRooms) == 0 {
			break
		}

		// Spawn the key in any available room
		keyRoom := availableRooms[ctx.Rng().Intn(len(availableRooms))]
		keyPos, _ := m.TakeRandomItemPoint(keyRoom.r, ctx.Rng())
		keyId := ctx.Playthrough.GetId()
		key := model.NewKeyItem(model.ItemId(keyId), keyPos, curKeyhole)
		ctx.Playthrough.AddItem(key)
		usedCells[keyRoom]++

		for _, edge := range curLockedEdges {
			m.LockDoor(edge.door.Pos, curKeyhole)
		}
		curKeyhole++
	}
}

// getDoorsForKey - returns number of doors which key can block for every key
func getDoorsForKey(doorsCount, keysCount int, rng *rand.Rand) []int {
	doorsForKey := make([]int, keysCount)
	for i := range keysCount {
		doorsForKey[i] = doorsCount / keysCount
	}

	perm := rng.Perm(keysCount)
	for i := 0; i < doorsCount%keysCount; i++ {
		doorsForKey[perm[i]]++
	}

	return doorsForKey
}

//
//
// --- NAVIGATION MAP ---
//
//

// NavMap - structure for simplified map representation, where rooms and corridors are nodes, doors are edges
type NavMap struct {
	graph       map[NavNode][]*Edge
	rooms       []*RoomWrp
	hubs        []*CorridorHub
	edgeCounter int64
}

// NavNode - interface for nodes
type NavNode interface {
	GetItemCapacity() int
	GetOpenDoorCount() int
}

// Edge - structure for graph edges
type Edge struct {
	id   int64
	src  NavNode
	door *model.DoorMetadata
	dst  NavNode
	back *Edge // If graph is indirect, should store the pointer to backward edge
}

//
// -- CONSTRUCTOR --
//

func NewNavMap(rooms []*model.Room) *NavMap {
	nm := &NavMap{
		graph: make(map[NavNode][]*Edge),
		rooms: make([]*RoomWrp, 0, len(rooms)),
		hubs:  make([]*CorridorHub, 0, len(rooms)),
	}

	for _, room := range rooms {
		nm.rooms = append(nm.rooms, &RoomWrp{r: room})
	}

	return nm
}

//
// -- EDGE METHODS --
//

func (nm *NavMap) createEdge(src NavNode, dst NavNode, door *model.DoorMetadata) {
	abEdge := &Edge{id: nm.edgeCounter, src: src, door: door, dst: dst}
	baEdge := &Edge{id: nm.edgeCounter, src: dst, door: door, dst: src}
	abEdge.back = baEdge
	baEdge.back = abEdge
	nm.edgeCounter++

	nm.graph[src] = append(nm.graph[src], abEdge)
	nm.graph[dst] = append(nm.graph[dst], baEdge)
	appendEdge(src, abEdge)
	appendEdge(dst, baEdge)
}

func appendEdge(node NavNode, edge *Edge) {
	switch n := node.(type) {
	case *RoomWrp:
		n.edges = append(n.edges, edge)
	case *CorridorHub:
		n.edges = append(n.edges, edge)
	}
}

func (nm *NavMap) deleteEdge(edge *Edge) {
	nm.removeEdgeFromGraph(edge.src, edge)
	removeEdgeFromNode(edge.src, edge)
	nm.removeEdgeFromGraph(edge.dst, edge.back)
	removeEdgeFromNode(edge.dst, edge.back)
}

func (nm *NavMap) removeEdgeFromGraph(node NavNode, edge *Edge) {
	for i, e := range nm.graph[node] {
		if e == edge {
			nm.graph[node] = algorithm.Remove(nm.graph[node], i)
			break
		}
	}
}

func removeEdgeFromNode(node NavNode, edge *Edge) {
	switch n := node.(type) {
	case *RoomWrp:
		for i, e := range n.edges {
			if e == edge {
				n.edges = append(n.edges[:i], n.edges[i+1:]...)
				return
			}
		}
	case *CorridorHub:
		for i, e := range n.edges {
			if e == edge {
				n.edges = append(n.edges[:i], n.edges[i+1:]...)
				return
			}
		}
	}
}

func (nm *NavMap) findEntrance(entranceId model.RoomId) *RoomWrp {
	for _, room := range nm.rooms {
		if room.r.Id == entranceId {
			return room
		}
	}
	return nil
}

func (nm *NavMap) getAvailableEdges(topology *TopologyData, startRoom *RoomWrp) []*Edge {
	availableEdges := make([]*Edge, 0, len(topology.Bridges))
	for edge := range topology.Bridges {
		// Don't block doors in start room
		if edge.src != startRoom && edge.dst != startRoom {
			availableEdges = append(availableEdges, edge)
		}
	}
	// For determinism of test
	slices.SortFunc(availableEdges, func(a, b *Edge) int {
		if a.id < b.id {
			return 1
		} else if a.id > b.id {
			return -1
		}
		return 0
	})
	return availableEdges
}

//
// -- NAV NODE INTERFACE IMPLEMENTATION ---
//

type RoomWrp struct {
	r     *model.Room
	edges []*Edge
}

// GetItemCapacity - returns room's number of empty points for items
func (r *RoomWrp) GetItemCapacity() int {
	return min(MaxKeysForRoom, max(len(r.r.GetEmptyItemPoints()), 0))
}

func (r *RoomWrp) GetOpenDoorCount() int {
	openDoorCount := 0
	for _, edge := range r.edges {
		if !edge.door.Locked {
			openDoorCount++
		}
	}
	return openDoorCount
}

// CorridorHub - abstraction for connections between rooms
type CorridorHub struct {
	cells map[geometry.Point]struct{}
	edges []*Edge
}

// GetItemCapacity - returns number of available points for items in the corridor
func (ch *CorridorHub) GetItemCapacity() int {
	return 0
}

func (ch *CorridorHub) GetOpenDoorCount() int {
	return 0
}

//
// -- NAV MAP METHODS --
//

// TopologyData - structure for storing found topology data of map
type TopologyData struct {
	Bridges         map[*Edge]bool      // Slice of pointers to edges that are an only way between rooms on the map
	ParentMap       map[NavNode]NavNode // Hashmap with pointers to the parent for every node
	SubtreeCapacity map[NavNode]int     // Number of empty item cells in the subtree
	SubtreeDoors    map[NavNode]int     // Number of doors in the subtree
	EntryTime       map[NavNode]int     // Use to define what a node is closer to start
	ExitTime        map[NavNode]int     // Need to check if the node is in any subtree
}

func (topology *TopologyData) determineFurtherNode(edge *Edge) NavNode {
	if topology.EntryTime[edge.dst] > topology.EntryTime[edge.src] {
		return edge.dst
	}
	return edge.src
}

func (nm *NavMap) analyzeTopology(src NavNode, usedCells map[NavNode]int) *TopologyData {
	data := &TopologyData{
		Bridges:         make(map[*Edge]bool),
		ParentMap:       make(map[NavNode]NavNode),
		SubtreeCapacity: make(map[NavNode]int),
		SubtreeDoors:    make(map[NavNode]int),
		EntryTime:       make(map[NavNode]int),
		ExitTime:        make(map[NavNode]int),
	}

	timer := 0

	discovery := make(map[NavNode]int)
	lowLink := make(map[NavNode]int)

	var dfs func(cur NavNode, incomingEdge *Edge)
	dfs = func(cur NavNode, incomingEdge *Edge) {
		discovery[cur] = timer // In what order current node was discovered
		lowLink[cur] = timer   // Which node is the closest to the start we can go to from this node directly
		data.EntryTime[cur] = timer
		timer++

		if cur == src {
			data.SubtreeCapacity[cur] = 0
		} else {
			used, _ := usedCells[cur]
			data.SubtreeCapacity[cur] = max(0, cur.GetItemCapacity()-used)
		}

		if cur != src {
			data.SubtreeDoors[cur] = cur.GetOpenDoorCount() // Returns number of open doors (consider doors as part of the rooms, so for corridor return value will be 0)
		} else {
			data.SubtreeDoors[cur] = 0
		}

		if incomingEdge != nil {
			data.ParentMap[cur] = incomingEdge.src
		} else {
			data.ParentMap[cur] = nil
		}

		for _, edge := range nm.graph[cur] {
			neighbor := edge.dst

			// If checked the backward edge
			if incomingEdge != nil && edge == incomingEdge.back {
				continue
			}

			if _, visited := discovery[neighbor]; visited {
				// Update where we can go from this node
				lowLink[cur] = min(lowLink[cur], discovery[neighbor])
			} else {
				dfs(neighbor, edge)
				data.SubtreeCapacity[cur] += data.SubtreeCapacity[neighbor]
				data.SubtreeDoors[cur] += data.SubtreeDoors[neighbor]
				lowLink[cur] = min(lowLink[cur], lowLink[neighbor])

				// If neighbors node is only connected with current node, its lowLink will be higher
				if lowLink[neighbor] > discovery[cur] {
					data.Bridges[edge] = true
					data.Bridges[edge.back] = true
				}
			}
		}
		data.ExitTime[cur] = timer
	}

	dfs(src, nil)
	return data
}

// createNavMap - creates simplified map representation where rooms and corridors are nodes, doors are edges in the graph
func createNavMap(playthrough *model.Playthrough) *NavMap {
	m := playthrough.Map
	nm := NewNavMap(playthrough.Map.GetRooms())
	hubs := findCorridorHubs(playthrough)
	visitedDoors := make(map[geometry.Point]bool)

	for _, room := range nm.rooms {
		queue := []geometry.Point{room.r.Center}
		visited := map[geometry.Point]bool{room.r.Center: true}

		for len(queue) > 0 {
			cur := queue[0]
			queue = queue[1:]

			if tile, _ := m.GetTileType(cur); tile == model.OpenDoor && !visitedDoors[cur] {
				visitedDoors[cur] = true
				navNodes := nm.findSrcAndDst(m, hubs, cur)
				door := m.GetDoors()[cur]
				nm.createEdge(navNodes[0], navNodes[1], door)
				continue
			}

			for _, dir := range geometry.GetCardinalDirs() {
				neighbor := cur.Add(dir)
				if !m.IsWalkable(neighbor) || visited[neighbor] {
					continue
				}

				visited[neighbor] = true
				queue = append(queue, neighbor)
			}
		}
	}

	return nm
}

// findCorridorHubs - finds all corridors between rooms.
// CorridorHub can be simply connection of two rooms or the main artery of the map that unites many rooms
func findCorridorHubs(playthrough *model.Playthrough) []*CorridorHub {
	m := playthrough.Map
	visited := make(map[geometry.Point]bool)
	hubs := make([]*CorridorHub, 0)

	for r := 0; r < m.Height; r++ {
		for c := 0; c < m.Width; c++ {
			p := geometry.Point{X: c, Y: r}
			if tile, _ := m.GetTileType(geometry.Point{X: c, Y: r}); tile == model.Corridor && !visited[p] {
				hubPoints := floodFill(playthrough, p, visited)
				hubs = append(hubs, &CorridorHub{
					cells: hubPoints,
				})
			}
		}
	}

	return hubs
}

// floodFill - classic BFS flood fill algorithm
func floodFill(playthrough *model.Playthrough, start geometry.Point, visited map[geometry.Point]bool) map[geometry.Point]struct{} {
	m := playthrough.Map

	visited[start] = true
	queue := []geometry.Point{start}
	startTile, _ := m.GetTileType(start) // Fill in only the cells having the type of the first cell
	filled := map[geometry.Point]struct{}{}

	for len(queue) > 0 {
		cur := queue[0]
		queue = queue[1:]
		filled[cur] = struct{}{}

		for _, dir := range geometry.GetCardinalDirs() {
			n := cur.Add(dir)
			nTile, _ := m.GetTileType(n)
			if m.InBounds(n) && nTile == startTile && !visited[n] {
				visited[n] = true
				queue = append(queue, n)
			}
		}
	}

	return filled
}

func (nm *NavMap) findSrcAndDst(m *model.Map, hubs []*CorridorHub, start geometry.Point) []NavNode {
	navNodes := make([]NavNode, 0, 2)

	for _, dir := range geometry.GetCardinalDirs() {
		n := start.Add(dir)
		if !m.IsWalkable(n) {
			continue
		}
		room := nm.getRoomWrpById(m.GetTileGrid()[n.Y][n.X].RoomId)
		hub := getHubByPoint(n, hubs)

		if room != nil {
			navNodes = append(navNodes, room)
		} else if hub != nil {
			navNodes = append(navNodes, hub)
		}
	}

	return navNodes
}

func getHubByPoint(point geometry.Point, hubs []*CorridorHub) *CorridorHub {
	for _, ch := range hubs {
		if _, exists := ch.cells[point]; exists {
			return ch
		}
	}
	return nil
}

func (nm *NavMap) getRoomWrpById(id model.RoomId) *RoomWrp {
	for _, wrp := range nm.rooms {
		if wrp.r.Id == id {
			return wrp
		}
	}
	return nil
}
