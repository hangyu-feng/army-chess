package game_test

import (
	"encoding/json"
	"os"
	"testing"
	"time"

	"github.com/fenghangyu/army-chess/server/internal/game"
)

func TestBoard1v1ContractMatchesTheTraditionalTopology(t *testing.T) {
	raw, err := os.ReadFile("../../../contracts/board.1v1.json")
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
	board := game.NewBoardForMode(game.OneVsOne)
	if contract.Version != board.Version || contract.Width != board.Width || contract.Height != board.Height {
		t.Fatalf("contract/runtime mismatch: %#v vs %s %dx%d", contract, board.Version, board.Width, board.Height)
	}
	if len(board.Nodes) != 65 {
		t.Fatalf("1v1 board has %d nodes, want 65", len(board.Nodes))
	}
	for _, seat := range []game.Seat{game.North, game.South} {
		zone := 0
		camps := 0
		headquarters := 0
		for _, node := range board.Nodes {
			if node.DeployFor != seat {
				continue
			}
			zone++
			switch node.Type {
			case game.Camp:
				camps++
			case game.Headquarters:
				headquarters++
			}
		}
		if zone != 30 || camps != 5 || headquarters != 2 || len(board.DeploymentNodes(seat)) != 25 {
			t.Fatalf("%s zone counts: positions=%d camps=%d headquarters=%d deployment=%d", seat, zone, camps, headquarters, len(board.DeploymentNodes(seat)))
		}
	}
	for _, id := range []string{"frontline-1L", "frontline-3", "frontline-1R"} {
		node, ok := board.Node(id)
		if !ok || node.Type != game.Frontline {
			t.Fatalf("missing frontline node %s: %#v", id, node)
		}
	}
	for _, id := range []string{"mountain-2L", "mountain-2R"} {
		node, ok := board.Node(id)
		if !ok || node.Type != game.Mountain {
			t.Fatalf("missing mountain node %s: %#v", id, node)
		}
		if len(board.Adj[id]) != 0 {
			t.Fatalf("mountain %s must not be connected: %#v", id, board.Adj[id])
		}
	}
}

func TestBoard1v1HasThreeIndependentFrontlineRailways(t *testing.T) {
	board := game.NewBoardForMode(game.OneVsOne)
	for _, edge := range []struct{ from, to string }{
		{"north-r1-1L", "frontline-1L"}, {"frontline-1L", "south-r1-1L"},
		{"north-r1-3", "frontline-3"}, {"frontline-3", "south-r1-3"},
		{"north-r1-1R", "frontline-1R"}, {"frontline-1R", "south-r1-1R"},
	} {
		found, ok := board.DirectEdge(edge.from, edge.to)
		if !ok || found.Type != "rail" {
			t.Fatalf("missing frontline railway %s -> %s: %#v, found=%t", edge.from, edge.to, found, ok)
		}
	}
	for _, edge := range []struct{ from, to string }{
		{"frontline-1L", "frontline-3"}, {"frontline-3", "frontline-1R"},
		{"mountain-2L", "frontline-1L"}, {"mountain-2R", "frontline-1R"},
	} {
		if _, ok := board.DirectEdge(edge.from, edge.to); ok {
			t.Fatalf("unexpected central-band connection %s -> %s", edge.from, edge.to)
		}
	}
}

func TestOneVsOneDefaultDeploymentUsesTheDedicatedZones(t *testing.T) {
	board := game.NewBoardForMode(game.OneVsOne)
	for _, seat := range []game.Seat{game.North, game.South} {
		deployment := game.DefaultDeployment(board, seat)
		if err := game.ValidateDeployment(board, seat, deployment); err != nil {
			t.Fatalf("%s default deployment: %v", seat, err)
		}
		if len(deployment) != 25 {
			t.Fatalf("%s deployment has %d pieces", seat, len(deployment))
		}
		for node, piece := range deployment {
			position, ok := board.Node(node)
			if !ok || position.DeployFor != seat || position.Type == game.Camp || position.Type == game.Frontline || position.Type == game.Mountain {
				t.Fatalf("%s piece is outside its 1v1 deployment zone: %s", seat, node)
			}
			if piece.Kind == game.Mine && position.Row < 5 {
				t.Fatalf("%s mine was placed above the rear two rows: %s row=%d", seat, node, position.Row)
			}
			if piece.Kind == game.Bomb && position.Row == 1 {
				t.Fatalf("%s bomb was placed in the front row: %s", seat, node)
			}
		}
	}
}

func TestOneVsOneRailwayCrossingAndMountainRejection(t *testing.T) {
	board := game.NewBoardForMode(game.OneVsOne)
	now := time.Now().UTC()
	for _, kind := range []game.PieceKind{game.Company, game.Engineer} {
		state := game.NewStateWithMatchMode(game.OneVsOne, game.FourDark, game.Fast, game.North, now)
		state.Phase = game.Playing
		state.Turn = game.North
		state.Players[game.North] = game.Player{Username: "north"}
		state.Players[game.South] = game.Player{Username: "south"}
		state.Pieces["north-r1-1L"] = game.Piece{ID: "north-piece", Owner: game.North, Kind: kind}

		moves := state.LegalMoves(board, game.North)
		contains := func(want string) bool {
			for _, move := range moves {
				if move == want {
					return true
				}
			}
			return false
		}
		if !contains("north-r1-1L->south-r1-1L") {
			t.Fatalf("%s should cross its uninterrupted frontline railway: %v", kind, moves)
		}
		turningMove := "north-r1-1L->south-r1-3"
		if kind == game.Engineer && !contains(turningMove) {
			t.Fatalf("engineer should turn after crossing the frontline: %v", moves)
		}
		if kind != game.Engineer && contains(turningMove) {
			t.Fatalf("ordinary piece turned at the frontline: %v", moves)
		}
		if contains("north-r1-1L->mountain-2L") {
			t.Fatalf("mountain was offered as a legal destination: %v", moves)
		}
	}
}
