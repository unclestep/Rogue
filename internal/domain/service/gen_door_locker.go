package service

import (
	"log"
	"math/rand"
	"slices"

	"github.com/unclestep/Rogue/internal/domain/model"
	"github.com/unclestep/Rogue/pkg/geometry"
	"github.com/unclestep/Rogue/pkg/utils"
)

// DoorLocker - service which locks doors on the map
type DoorLocker struct{}

func NewDoorLocker() *DoorLocker {
	return &DoorLocker{}
}

//
//
// --- NAVIGATION MAP ---
//
//

// NavMap - structure for simplified map representation, where rooms and corridors are nodes, doors are edges
type NavMap struct {
	graph map[NavNode][]*Edge
	rooms []*RoomWrp
	hubs  []*CorridorHub
}

// NavNode - interface for nodes
type NavNode interface {
	GetID() int
	GetCapacity() int
	GetOpenDoorsCount() int
}

// Edge - structure for graph edges
type Edge struct {
	id   int
	src  NavNode
	door *model.DoorMetadata
	dst  NavNode
	back *Edge // If graph is indirect, should store the pointer to backward edge
}

func NewNavMap() *NavMap {
	return &NavMap{
		graph: make(map[NavNode][]*Edge),
		rooms: make([]*RoomWrp, 0, 9),
		hubs:  make([]*CorridorHub, 0, 9),
	}
}

//
// -- NAV NODE INTERFACE IMPLEMENTATION ---
//

type RoomWrp struct {
	r *model.Room
}

// GetID - returns room ID
func (r *RoomWrp) GetID() int {
	return int(r.r.Id)
}

// GetCapacity - returns room's number of empty points for items
func (r *RoomWrp) GetCapacity() int {
	return MaxKeysForRoom
}

// GetOpenDoorsCount - returns number of open doors in the room
func (r *RoomWrp) GetOpenDoorsCount() int {
	openDoors := 0

	for _, d := range r.r.GetDoors() {
		if !d.Locked {
			openDoors++
		}
	}

	return openDoors
}

// CorridorHub - abstraction for connections between rooms
type CorridorHub struct {
	id    int
	cells map[geometry.Point]struct{}
}

// GetID - returns corridor hub's ID
func (ch *CorridorHub) GetID() int {
	return ch.id
}

// GetCapacity - returns number of available points for items in the corridor
func (ch *CorridorHub) GetCapacity() int {
	return 0
}

// GetOpenDoorsCount - returns number of open doors in the corridors
func (ch *CorridorHub) GetOpenDoorsCount() int {
	return 0 // 0, because doors are belong to rooms
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
	lowLink := make(map[NavNode]int) // To

	var dfs func(cur NavNode, incomingEdge *Edge)
	dfs = func(cur NavNode, incomingEdge *Edge) {
		discovery[cur] = timer // In what order current node was discovered
		lowLink[cur] = timer   // Which node is the closest to the start we can go to from this node directly
		data.EntryTime[cur] = timer
		timer++

		curCapacity := cur.GetCapacity()
		if cur == src {
			curCapacity = 0
		} else if used, exists := usedCells[cur]; exists && used > 0 {
			curCapacity -= used
			if curCapacity < 0 {
				curCapacity = 0
			}
		}
		data.SubtreeCapacity[cur] = curCapacity

		if cur != src {
			data.SubtreeDoors[cur] = cur.GetOpenDoorsCount() // Returns number of open doors (consider doors as part of the rooms, so for corridor return value will be 0)
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
func (d *DoorLocker) createNavMap(playthrough *model.Playthrough) *NavMap {
	m := playthrough.Map

	nm := NewNavMap()
	hubs := d.findCorridorHubs(playthrough)
	edgeID := 1

	for _, room := range m.GetRooms() {
		for _, door := range room.GetDoors() {
			hub := d.getHubConnectedToDoor(playthrough, hubs, door.Pos)
			if hub != nil {
				rwrp := &RoomWrp{r: room}

				abEdge := &Edge{
					id:   edgeID,
					src:  rwrp,
					door: door,
					dst:  hub,
				}
				edgeID++

				// Graph is indirect, don't forget to create backward edge
				baEdge := &Edge{
					id:   edgeID,
					src:  hub,
					door: door,
					dst:  rwrp,
				}
				edgeID++

				abEdge.back = baEdge
				baEdge.back = abEdge

				nm.graph[rwrp] = append(nm.graph[rwrp], abEdge)
				nm.graph[hub] = append(nm.graph[hub], baEdge)
				nm.rooms = append(nm.rooms, rwrp)
			}
		}
	}

	return nm
}

// findCorridorHubs - finds all corridors between rooms.
// CorridorHub can be simply connection of two rooms or the main artery of the map that unites many rooms
func (d *DoorLocker) findCorridorHubs(playthrough *model.Playthrough) []*CorridorHub {
	m := playthrough.Map
	visited := make(map[geometry.Point]bool)
	hubs := make([]*CorridorHub, 0)
	id := len(playthrough.Map.GetRooms())

	for r := 0; r < m.Height; r++ {
		for c := 0; c < m.Width; c++ {
			p := geometry.Point{X: c, Y: r}

			if tile, _ := m.GetTileType(geometry.Point{X: c, Y: r}); tile == model.Corridor && !visited[p] {
				hubPoints := d.floodFill(playthrough, p, visited)
				hubs = append(hubs, &CorridorHub{
					id:    id,
					cells: hubPoints,
				})
				id++
			}
		}
	}

	return hubs
}

// floodFill - classic BFS flood fill algorithm
func (d *DoorLocker) floodFill(playthrough *model.Playthrough, start geometry.Point, visited map[geometry.Point]bool) map[geometry.Point]struct{} {
	m := playthrough.Map

	visited[start] = true
	queue := []geometry.Point{start}
	startTile, _ := m.GetTileType(start) // Fill in only the cells having the type of the first cell

	filled := map[geometry.Point]struct{}{}

	for len(queue) > 0 {
		cur := queue[0]
		queue = queue[1:]
		filled[cur] = struct{}{}

		for _, dir := range geometry.GetAllDirs() {
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

// TODO: Consider situation when we do not have corridors and rooms are connected only by door
// getHubConnectedToDoor - returns pointer to CorridorHub that runs into the door
func (d *DoorLocker) getHubConnectedToDoor(session *model.Playthrough, hubs []*CorridorHub, door geometry.Point) *CorridorHub {
	m := session.Map

	for _, dir := range geometry.GetCardinalDirs() {
		n := door.Add(dir)
		if m.InBounds(n) {
			for _, hub := range hubs {
				if _, exists := hub.cells[n]; exists {
					return hub
				}
			}
		}
	}

	return nil
}

func (nm NavMap) deleteEdge(edge *Edge) {
	srcNode := edge.src
	srcEdges := nm.graph[srcNode]
	removeInd := -1

	for i, e := range srcEdges {
		if e == edge {
			removeInd = i
			break
		}
	}

	if removeInd != -1 {
		srcEdges[removeInd] = srcEdges[len(srcEdges)-1]
		srcEdges = srcEdges[:len(srcEdges)-1]
		nm.graph[srcNode] = srcEdges
	}

	backEdge := edge.back
	dstNode := edge.dst
	dstEdges := nm.graph[dstNode]
	removeInd = -1

	for i, e := range dstEdges {
		if e == backEdge {
			removeInd = i
			break
		}
	}

	if removeInd != -1 {
		dstEdges[removeInd] = dstEdges[len(dstEdges)-1]
		dstEdges = dstEdges[:len(dstEdges)-1]
		nm.graph[dstNode] = dstEdges
	}
}

const (
	MaxKeysForRoom = 1
)

// GenerateKeysAndDoors - generates doorsCount doors and keysCount keys.
// Returns map[keyID]keyPos.
// If doorsCount > keysCount, some keys can open more than one door;
// if keysCount > doorsCount, keysCount = doorsCount;
// if doorsCount is greater than total number of doors, algorithm will block all found bridges on the map;
// if doorsCount < 0 || keysCount < 0, nil will be returned.
// Algorithm does not block loop edges.
func (d *DoorLocker) LockDoors(ctx *model.SessionContext, doorsCount, keysCount int) {
	if doorsCount <= 0 || keysCount <= 0 {
		log.Printf("[INFO] Given doorsCount=%d, keysCount=%d but expected doorsCount>0 and keysCount>0", doorsCount, keysCount)
		return
	}

	m := ctx.Playthrough.Map

	if m == nil {
		log.Printf("[ERROR] Map topology was not generated yet\n")
		return
	}

	usedCells := make(map[NavNode]int)

	nm := d.createNavMap(ctx.Playthrough)
	entranceRoom := nm.rooms[0]
	for _, room := range nm.rooms {
		if room.r.Id == m.EntranceRoomId {
			entranceRoom = room
			break
		}
	}
	topology := nm.analyzeTopology(entranceRoom, usedCells)

	getAvailableEdges := func(topology *TopologyData) []*Edge {
		availableEdges := make([]*Edge, 0, len(topology.Bridges))
		for edge := range topology.Bridges {
			// Don't block doors in start room
			if edge.src != entranceRoom && edge.dst != entranceRoom {
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
		// Make the order random using local random generator
		utils.Shuffle(ctx.Rng(), availableEdges)
		return availableEdges
	}

	validKeysCount := min(doorsCount, keysCount)
	validDoorsCount := min(validKeysCount, len(m.GetDoors()))
	doorsForKey := d.getDoorsForKey(validDoorsCount, validKeysCount, ctx.Rng())

	var curKeyhole model.Keyhole = 1

	for k := range validKeysCount {
		numDoors := doorsForKey[k]          // Get number of doors to be blocked by current key
		keysRemaining := validKeysCount - k // Keys left

		curLockedEdges := make([]*Edge, 0) // Slice of locked edges by current key

		// Start blocking the doors
		for range numDoors {
			availableEdges := getAvailableEdges(topology)

			totalCapacity := topology.SubtreeCapacity[entranceRoom]
			totalDoors := topology.SubtreeDoors[entranceRoom]

			if len(availableEdges) == 0 {
				break
			}

			// Edge that satisfies all conditions
			var selectedEdge *Edge

			// Edge that does not satisfy all conditions but looks convenient to be locked
			var optimalEdge *Edge
			maxOptimalDoors := -1

			// Find convenient room in the path
			for _, edge := range availableEdges {
				var node NavNode
				// It is integral to correct define the node which will be locked
				// Unavailable node is always the one that's further from the center
				if topology.EntryTime[edge.dst] > topology.EntryTime[edge.src] {
					node = edge.dst
				} else {
					node = edge.src
				}

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
				if availableCells >= keysRemaining && availableDoors > maxOptimalDoors {
					maxOptimalDoors = availableDoors
					optimalEdge = edge
				}
			}

			// If we couldn't find the edge that satisfies all the conditions, take the optimal one
			if selectedEdge == nil {
				// If optimal edge is not found either, stop the loop
				if optimalEdge == nil {
					break
				}
				selectedEdge = optimalEdge
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
		availableRooms := make([]*RoomWrp, 0, len(m.GetRooms()))
		for _, r := range nm.rooms {
			// Check if the room is in locked subtree
			if _, reachable := topology.EntryTime[r]; !reachable {
				continue
			}

			// If there is no space or consumed all the available cells
			if r == entranceRoom || usedCells[r] >= MaxKeysForRoom {
				continue
			}

			isCritical := true

			// If key is last
			if keysRemaining == 1 {
				isCritical = false
			} else {
				// We are looking at how the spawn will affect subsequent locks
				usedCells[r]++
				testTopology := nm.analyzeTopology(entranceRoom, usedCells)
				totalCapacity := testTopology.SubtreeCapacity[entranceRoom]

				for bridge := range testTopology.Bridges {
					var node NavNode
					if testTopology.EntryTime[bridge.dst] > testTopology.EntryTime[bridge.src] {
						node = bridge.dst
					} else {
						node = bridge.src
					}

					cutCells := testTopology.SubtreeCapacity[node]
					availableCells := totalCapacity - cutCells

					// If we have found at least one bridge that can be safely blocked, then the room is safe.
					if availableCells >= (keysRemaining - 1) {
						isCritical = false
						break
					}
				}
				usedCells[r]--
			}

			if !isCritical {
				availableRooms = append(availableRooms, r)
			}
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
		topology = nm.analyzeTopology(entranceRoom, usedCells)

		// Truly lock all the selected edges
		for _, edge := range curLockedEdges {
			door := edge.door

			// Lock the door
			door.Locked = true
			door.Keyhole = curKeyhole
			door.KeyPos = keyPos // Save the door key position
			m.LockDoor(door.Pos)
		}

		curKeyhole++
	}
}

// getDoorsForKey - returns number of doors which key can block for every key
func (d *DoorLocker) getDoorsForKey(doorsCount, keysCount int, rng *rand.Rand) []int {
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
