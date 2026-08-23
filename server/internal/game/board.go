package game

import "sort"

type NodeType string

const (
	Station      NodeType = "station"
	Camp         NodeType = "camp"
	Headquarters NodeType = "headquarters"
)

type Node struct {
	ID        string   `json:"id"`
	X         float64  `json:"x"`
	Y         float64  `json:"y"`
	Type      NodeType `json:"type"`
	DeployFor Seat     `json:"deployFor,omitempty"`
	Row       int      `json:"row,omitempty"`
	Column    string   `json:"column,omitempty"`
}

type RailHeading string

const (
	RailNorth RailHeading = "north"
	RailEast  RailHeading = "east"
	RailSouth RailHeading = "south"
	RailWest  RailHeading = "west"
)

type Edge struct {
	From            string      `json:"from"`
	To              string      `json:"to"`
	Type            string      `json:"type"`
	RailwayOrigin   RailHeading `json:"railwayOrigin,omitempty"`
	RailwayTerminal RailHeading `json:"railwayTerminal,omitempty"`
}

type BoardDefinition struct {
	Version string          `json:"version"`
	Width   int             `json:"width"`
	Height  int             `json:"height"`
	Nodes   map[string]Node `json:"nodes"`
	Edges   []Edge          `json:"edges"`
}

type Board struct {
	Version string            `json:"version"`
	Width   int               `json:"width"`
	Height  int               `json:"height"`
	Nodes   map[string]Node   `json:"nodes"`
	Edges   []Edge            `json:"edges"`
	Adj     map[string][]Edge `json:"-"`
}

var localColumns = []string{"1L", "2L", "3", "2R", "1R"}

type localPosition struct {
	row int
	col int
}

// NewBoard creates the standard four-country Army Chess topology. Coordinates
// are drawing coordinates in a 1000x1000 view box; movement is determined only
// by the explicit graph below.
func NewBoard() *Board {
	b := &Board{
		Version: "board.v2",
		Width:   1000,
		Height:  1000,
		Nodes:   map[string]Node{},
		Adj:     map[string][]Edge{},
	}

	for _, seat := range Seats {
		for row := 1; row <= 6; row++ {
			for col := range localColumns {
				x, y := localCoordinates(row, col)
				x, y = rotateCoordinates(seat, x, y)
				nodeType := Station
				switch {
				case (row == 2 || row == 4) && (col == 1 || col == 3), row == 3 && col == 2:
					nodeType = Camp
				case row == 6 && (col == 1 || col == 3):
					nodeType = Headquarters
				}
				id := localNodeID(seat, row, col)
				b.Nodes[id] = Node{ID: id, X: x, Y: y, Type: nodeType, DeployFor: seat, Row: row, Column: localColumns[col]}
			}
		}
	}

	palace := map[string][2]float64{
		"palace-nw": {410, 410}, "palace-n": {500, 410}, "palace-ne": {590, 410},
		"palace-w": {410, 500}, "palace-c": {500, 500}, "palace-e": {590, 500},
		"palace-sw": {410, 590}, "palace-s": {500, 590}, "palace-se": {590, 590},
	}
	for id, point := range palace {
		b.Nodes[id] = Node{ID: id, X: point[0], Y: point[1], Type: Station}
	}

	for _, seat := range Seats {
		for _, row := range []int{2, 3, 4, 6} {
			for col := 0; col < 4; col++ {
				b.addRoad(localNodeID(seat, row, col), localNodeID(seat, row, col+1))
			}
		}
		for _, col := range []int{1, 2, 3} {
			for row := 2; row < 6; row++ {
				b.addRoad(localNodeID(seat, row, col), localNodeID(seat, row+1, col))
			}
		}
		for _, col := range []int{0, 4} {
			b.addRoad(localNodeID(seat, 5, col), localNodeID(seat, 6, col))
		}
		for _, row := range []int{1, 5} {
			for col := 0; col < 4; col++ {
				b.addRailBetween(localNodeID(seat, row, col), localNodeID(seat, row, col+1))
			}
		}
		for _, col := range []int{0, 4} {
			for row := 1; row < 5; row++ {
				b.addRailBetween(localNodeID(seat, row, col), localNodeID(seat, row+1, col))
			}
		}

		// The diagonal cross around every camp is a road junction. This includes
		// the five camps themselves; a camp is only protected when occupied.
		for _, camp := range []localPosition{{2, 1}, {2, 3}, {3, 2}, {4, 1}, {4, 3}} {
			for _, diagonal := range []localPosition{
				{camp.row - 1, camp.col - 1}, {camp.row - 1, camp.col + 1},
				{camp.row + 1, camp.col - 1}, {camp.row + 1, camp.col + 1},
			} {
				b.addRoad(localNodeID(seat, camp.row, camp.col), localNodeID(seat, diagonal.row, diagonal.col))
			}
		}
	}

	// The nine-palace is a 3x3 railway graph.
	palaceRows := [][]string{{"palace-nw", "palace-n", "palace-ne"}, {"palace-w", "palace-c", "palace-e"}, {"palace-sw", "palace-s", "palace-se"}}
	for row := range palaceRows {
		for col := 0; col < 2; col++ {
			b.addRailBetween(palaceRows[row][col], palaceRows[row][col+1])
		}
	}
	for col := range palaceRows[0] {
		for row := 0; row < 2; row++ {
			b.addRailBetween(palaceRows[row][col], palaceRows[row+1][col])
		}
	}

	interfaces := map[Seat]map[int]string{
		North: {0: "palace-ne", 2: "palace-n", 4: "palace-nw"},
		East:  {0: "palace-se", 2: "palace-e", 4: "palace-ne"},
		South: {0: "palace-sw", 2: "palace-s", 4: "palace-se"},
		West:  {0: "palace-nw", 2: "palace-w", 4: "palace-sw"},
	}
	for _, seat := range Seats {
		for col, palaceID := range interfaces[seat] {
			b.addRailBetween(localNodeID(seat, 1, col), palaceID)
		}
	}

	// These four bent routes connect neighboring front corners. Their headings
	// are retained so ordinary railway pieces cannot turn at the corner.
	b.addBentRail(localNodeID(South, 1, 4), localNodeID(East, 1, 0), RailNorth, RailEast)
	b.addBentRail(localNodeID(East, 1, 4), localNodeID(North, 1, 0), RailWest, RailNorth)
	b.addBentRail(localNodeID(North, 1, 4), localNodeID(West, 1, 0), RailSouth, RailWest)
	b.addBentRail(localNodeID(West, 1, 4), localNodeID(South, 1, 0), RailEast, RailSouth)

	return b
}

func localCoordinates(row, col int) (float64, float64) {
	return 406 + float64(col)*47, 648 + float64(row-1)*47
}

func rotateCoordinates(seat Seat, x, y float64) (float64, float64) {
	switch seat {
	case North:
		return 1000 - x, 1000 - y
	case East:
		return y, 1000 - x
	case West:
		return 1000 - y, x
	default:
		return x, y
	}
}

func localNodeID(seat Seat, row, col int) string {
	return string(seat) + "-r" + string(rune('0'+row)) + "-" + localColumns[col]
}

func (b *Board) addDirected(edge Edge) {
	for _, existing := range b.Adj[edge.From] {
		if existing.To == edge.To && existing.Type == edge.Type {
			return
		}
	}
	b.Edges = append(b.Edges, edge)
	b.Adj[edge.From] = append(b.Adj[edge.From], edge)
}

func (b *Board) addRoad(from, to string) {
	b.addDirected(Edge{From: from, To: to, Type: "road"})
	b.addDirected(Edge{From: to, To: from, Type: "road"})
}

func (b *Board) addRailBetween(from, to string) {
	fromNode, fromOK := b.Nodes[from]
	toNode, toOK := b.Nodes[to]
	if !fromOK || !toOK {
		return
	}
	heading := headingBetween(fromNode, toNode)
	b.addDirected(Edge{From: from, To: to, Type: "rail", RailwayOrigin: heading, RailwayTerminal: heading})
	b.addDirected(Edge{From: to, To: from, Type: "rail", RailwayOrigin: oppositeHeading(heading), RailwayTerminal: oppositeHeading(heading)})
}

func (b *Board) addBentRail(from, to string, first, second RailHeading) {
	b.addDirected(Edge{From: from, To: to, Type: "rail", RailwayOrigin: first, RailwayTerminal: second})
	b.addDirected(Edge{From: to, To: from, Type: "rail", RailwayOrigin: oppositeHeading(second), RailwayTerminal: oppositeHeading(first)})
}

func headingBetween(from, to Node) RailHeading {
	if from.X == to.X {
		if to.Y < from.Y {
			return RailNorth
		}
		return RailSouth
	}
	if to.X < from.X {
		return RailWest
	}
	return RailEast
}

func oppositeHeading(heading RailHeading) RailHeading {
	switch heading {
	case RailNorth:
		return RailSouth
	case RailEast:
		return RailWest
	case RailSouth:
		return RailNorth
	default:
		return RailEast
	}
}

func (b *Board) Definition() BoardDefinition {
	nodes := make(map[string]Node, len(b.Nodes))
	for id, node := range b.Nodes {
		nodes[id] = node
	}
	edges := append([]Edge(nil), b.Edges...)
	return BoardDefinition{Version: b.Version, Width: b.Width, Height: b.Height, Nodes: nodes, Edges: edges}
}

func (b *Board) DeploymentNodes(seat Seat) []string {
	ids := make([]string, 0, 25)
	for id, node := range b.Nodes {
		if node.DeployFor == seat && node.Type != Camp {
			ids = append(ids, id)
		}
	}
	sort.Strings(ids)
	return ids
}

func (b *Board) Node(id string) (Node, bool) { node, ok := b.Nodes[id]; return node, ok }

func (b *Board) DirectEdge(from, to string) (Edge, bool) {
	for _, edge := range b.Adj[from] {
		if edge.To == to {
			return edge, true
		}
	}
	return Edge{}, false
}
