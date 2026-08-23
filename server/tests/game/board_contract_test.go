package game_test

import (
	"encoding/json"
	"os"
	"testing"

	"github.com/fenghangyu/army-chess/server/internal/game"
)

func TestBoardV2ContractMatchesRuntimeTopology(t *testing.T) {
	raw, err := os.ReadFile("../../../contracts/board.v2.json")
	if err != nil {
		t.Fatal(err)
	}
	var contract struct {
		Version string `json:"version"`
		Width   int    `json:"width"`
		Height  int    `json:"height"`
	}
	if err := json.Unmarshal(raw, &contract); err != nil {
		t.Fatal(err)
	}
	board := game.NewBoard()
	if contract.Version != board.Version || contract.Width != board.Width || contract.Height != board.Height {
		t.Fatalf("contract/runtime mismatch: %#v vs %s %dx%d", contract, board.Version, board.Width, board.Height)
	}
	if len(board.Nodes) != 129 || len(board.Edges) == 0 {
		t.Fatalf("invalid generated topology: nodes=%d edges=%d", len(board.Nodes), len(board.Edges))
	}
	type zoneCounts struct{ total, camps, headquarters, deployment int }
	counts := map[game.Seat]zoneCounts{}
	for _, node := range board.Nodes {
		if !node.DeployFor.Valid() {
			continue
		}
		count := counts[node.DeployFor]
		count.total++
		switch node.Type {
		case game.Camp:
			count.camps++
		case game.Headquarters:
			count.headquarters++
		}
		counts[node.DeployFor] = count
	}
	for _, seat := range game.Seats {
		if got := len(board.DeploymentNodes(seat)); got != 25 {
			t.Fatalf("%s deployment has %d nodes", seat, got)
		}
		count := counts[seat]
		count.deployment = len(board.DeploymentNodes(seat))
		if count.total != 30 || count.camps != 5 || count.headquarters != 2 || count.deployment != 25 {
			t.Fatalf("%s zone counts: %#v", seat, count)
		}
	}
}

func TestBoardHasNoDuplicateEdgesAndPreservesRailwayHeadings(t *testing.T) {
	board := game.NewBoard()
	if len(board.Edges) != 552 {
		road, rail := 0, 0
		for _, edge := range board.Edges {
			if edge.Type == "road" {
				road++
			} else if edge.Type == "rail" {
				rail++
			}
		}
		t.Fatalf("unexpected edge count: got %d, want 552 (road=%d rail=%d)", len(board.Edges), road, rail)
	}
	seen := map[string]bool{}
	road, rail := 0, 0
	for _, edge := range board.Edges {
		key := edge.From + "|" + edge.To + "|" + edge.Type
		if seen[key] {
			t.Fatalf("duplicate directed edge: %s", key)
		}
		seen[key] = true
		switch edge.Type {
		case "road":
			road++
		case "rail":
			rail++
			if edge.RailwayOrigin == "" || edge.RailwayTerminal == "" {
				t.Fatalf("railway edge lacks heading metadata: %#v", edge)
			}
		default:
			t.Fatalf("unknown edge type %q", edge.Type)
		}
	}
	if road != 368 || rail != 184 {
		t.Fatalf("edge type counts: road=%d rail=%d", road, rail)
	}
	corner, ok := board.DirectEdge("south-r1-1R", "east-r1-1L")
	if !ok || corner.RailwayOrigin != game.RailNorth || corner.RailwayTerminal != game.RailEast {
		t.Fatalf("south-east corner route metadata: %#v, found=%t", corner, ok)
	}
	reverse, ok := board.DirectEdge("east-r1-1L", "south-r1-1R")
	if !ok || reverse.RailwayOrigin != game.RailWest || reverse.RailwayTerminal != game.RailSouth {
		t.Fatalf("reverse corner route metadata: %#v, found=%t", reverse, ok)
	}
}

func TestBoardRotatesLocalCountryCoordinates(t *testing.T) {
	board := game.NewBoard()
	expected := map[string][2]float64{
		"north-r1-1L": {590, 340},
		"east-r1-1L":  {660, 590},
		"south-r1-1L": {410, 660},
		"west-r1-1L":  {340, 410},
	}
	for id, want := range expected {
		node, ok := board.Node(id)
		if !ok || node.X != want[0] || node.Y != want[1] {
			t.Fatalf("%s coordinates: %#v, want (%v,%v), found=%t", id, node, want[0], want[1], ok)
		}
	}
}
