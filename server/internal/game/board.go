package game

import "fmt"

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
}

type Edge struct {
	From string `json:"from"`
	To   string `json:"to"`
	Type string `json:"type"`
}

type Board struct {
	Version string          `json:"version"`
	Width   int             `json:"width"`
	Height  int             `json:"height"`
	Nodes   map[string]Node `json:"nodes"`
	Edges   []Edge          `json:"edges"`
	Adj     map[string][]Edge
}

// NewBoard creates the stable v1 tactical grid. The four 5x5 corner zones are
// the 25 deployment positions for each seat; the remaining cells form the
// shared railway and camp area.
func NewBoard() *Board {
	b := &Board{Version: "board.v1", Width: 12, Height: 12, Nodes: map[string]Node{}, Adj: map[string][]Edge{}}
	for y := 0; y < b.Height; y++ {
		for x := 0; x < b.Width; x++ {
			id := fmt.Sprintf("n%02d_%02d", x, y)
			b.Nodes[id] = Node{ID: id, X: float64(x) / 11, Y: float64(y) / 11, Type: Station}
		}
	}
	zones := map[Seat][][2]int{
		North: {{0, 0}, {1, 0}, {2, 0}, {3, 0}, {4, 0}, {0, 1}, {1, 1}, {2, 1}, {3, 1}, {4, 1}, {0, 2}, {1, 2}, {2, 2}, {3, 2}, {4, 2}, {0, 3}, {1, 3}, {2, 3}, {3, 3}, {4, 3}, {0, 4}, {1, 4}, {2, 4}, {3, 4}, {4, 4}},
		East:  {{7, 0}, {8, 0}, {9, 0}, {10, 0}, {11, 0}, {7, 1}, {8, 1}, {9, 1}, {10, 1}, {11, 1}, {7, 2}, {8, 2}, {9, 2}, {10, 2}, {11, 2}, {7, 3}, {8, 3}, {9, 3}, {10, 3}, {11, 3}, {7, 4}, {8, 4}, {9, 4}, {10, 4}, {11, 4}},
		South: {{7, 7}, {8, 7}, {9, 7}, {10, 7}, {11, 7}, {7, 8}, {8, 8}, {9, 8}, {10, 8}, {11, 8}, {7, 9}, {8, 9}, {9, 9}, {10, 9}, {11, 9}, {7, 10}, {8, 10}, {9, 10}, {10, 10}, {11, 10}, {7, 11}, {8, 11}, {9, 11}, {10, 11}, {11, 11}},
		West:  {{0, 7}, {1, 7}, {2, 7}, {3, 7}, {4, 7}, {0, 8}, {1, 8}, {2, 8}, {3, 8}, {4, 8}, {0, 9}, {1, 9}, {2, 9}, {3, 9}, {4, 9}, {0, 10}, {1, 10}, {2, 10}, {3, 10}, {4, 10}, {0, 11}, {1, 11}, {2, 11}, {3, 11}, {4, 11}},
	}
	for seat, coords := range zones {
		for i, coord := range coords {
			id := fmt.Sprintf("n%02d_%02d", coord[0], coord[1])
			node := b.Nodes[id]
			node.DeployFor = seat
			if i == 0 || i == len(coords)-1 {
				node.Type = Headquarters
			}
			b.Nodes[id] = node
		}
	}
	for y := 0; y < b.Height; y++ {
		for x := 0; x < b.Width; x++ {
			id := fmt.Sprintf("n%02d_%02d", x, y)
			if b.Nodes[id].DeployFor != "" {
				continue
			}
			if (x == 5 || x == 6) && (y == 5 || y == 6) {
				node := b.Nodes[id]
				node.Type = Camp
				b.Nodes[id] = node
			}
		}
	}
	for y := 0; y < b.Height; y++ {
		for x := 0; x < b.Width; x++ {
			from := fmt.Sprintf("n%02d_%02d", x, y)
			if x+1 < b.Width {
				b.addEdge(from, fmt.Sprintf("n%02d_%02d", x+1, y), "rail")
			}
			if y+1 < b.Height {
				b.addEdge(from, fmt.Sprintf("n%02d_%02d", x, y+1), "rail")
			}
		}
	}
	return b
}

func (b *Board) addEdge(from, to, kind string) {
	b.Edges = append(b.Edges, Edge{From: from, To: to, Type: kind})
	b.Edges = append(b.Edges, Edge{From: to, To: from, Type: kind})
	b.Adj[from] = append(b.Adj[from], Edge{From: from, To: to, Type: kind})
	b.Adj[to] = append(b.Adj[to], Edge{From: to, To: from, Type: kind})
}

func (b *Board) DeploymentNodes(seat Seat) []string {
	ids := make([]string, 0, 25)
	for id, node := range b.Nodes {
		if node.DeployFor == seat {
			ids = append(ids, id)
		}
	}
	// IDs are generated in coordinate order, which is useful for a stable
	// default layout and for clients rendering the setup editor.
	for i := 1; i < len(ids); i++ {
		for j := i; j > 0 && ids[j] < ids[j-1]; j-- {
			ids[j], ids[j-1] = ids[j-1], ids[j]
		}
	}
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
