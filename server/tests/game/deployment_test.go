package game_test

import (
	"testing"
	"time"

	"github.com/fenghangyu/army-chess/server/internal/game"
)

func cloneDeployment(deployment map[string]game.Piece) map[string]game.Piece {
	copyDeployment := make(map[string]game.Piece, len(deployment))
	for node, piece := range deployment {
		copyDeployment[node] = piece
	}
	return copyDeployment
}

func findDeploymentNode(board *game.Board, deployment map[string]game.Piece, predicate func(game.Node, game.Piece) bool) string {
	for node, piece := range deployment {
		if position, ok := board.Node(node); ok && predicate(position, piece) {
			return node
		}
	}
	return ""
}

func swapKinds(deployment map[string]game.Piece, first, second string) {
	firstPiece, secondPiece := deployment[first], deployment[second]
	firstPiece.Kind, secondPiece.Kind = secondPiece.Kind, firstPiece.Kind
	deployment[first], deployment[second] = firstPiece, secondPiece
}

func TestDefaultDeploymentContainsExactlyThePublishedInventory(t *testing.T) {
	board := game.NewBoard()
	expected := map[game.PieceKind]int{}
	for _, kind := range game.Inventory() {
		expected[kind]++
	}
	for _, seat := range game.Seats {
		deployment := game.DefaultDeployment(board, seat)
		counts := map[game.PieceKind]int{}
		for node, piece := range deployment {
			counts[piece.Kind]++
			position, ok := board.Node(node)
			if !ok || position.Type == game.Camp || position.DeployFor != seat {
				t.Fatalf("%s default piece is outside its deployable zone: %s", seat, node)
			}
			if piece.Owner != seat {
				t.Fatalf("%s default piece has owner %s", seat, piece.Owner)
			}
		}
		if len(deployment) != 25 || len(counts) != len(expected) {
			t.Fatalf("%s default deployment size/counts: %d %#v", seat, len(deployment), counts)
		}
		for kind, want := range expected {
			if counts[kind] != want {
				t.Fatalf("%s %s count = %d, want %d", seat, kind, counts[kind], want)
			}
		}
		flag := findDeploymentNode(board, deployment, func(_ game.Node, piece game.Piece) bool { return piece.Kind == game.Flag })
		flagNode, _ := board.Node(flag)
		if flag == "" || flagNode.Type != game.Headquarters {
			t.Fatalf("%s flag is not in headquarters: %s", seat, flag)
		}
	}
}

func TestDeploymentRejectsAllSpecialPlacementViolations(t *testing.T) {
	board := game.NewBoard()
	base := game.DefaultDeployment(board, game.North)
	station := findDeploymentNode(board, base, func(node game.Node, piece game.Piece) bool {
		return node.Type == game.Station && piece.Kind != game.Flag && piece.Kind != game.Mine && piece.Kind != game.Bomb
	})
	flag := findDeploymentNode(board, base, func(_ game.Node, piece game.Piece) bool { return piece.Kind == game.Flag })
	mine := findDeploymentNode(board, base, func(_ game.Node, piece game.Piece) bool { return piece.Kind == game.Mine })
	bomb := findDeploymentNode(board, base, func(_ game.Node, piece game.Piece) bool { return piece.Kind == game.Bomb })
	front := findDeploymentNode(board, base, func(node game.Node, _ game.Piece) bool { return node.Row == 1 })
	if station == "" || flag == "" || mine == "" || bomb == "" || front == "" {
		t.Fatal("default deployment did not provide test positions")
	}

	tests := []struct {
		name      string
		mutate    func(map[string]game.Piece)
		wantValid bool
	}{
		{name: "flag outside headquarters", mutate: func(deployment map[string]game.Piece) { swapKinds(deployment, flag, station) }},
		{name: "mine in front row", mutate: func(deployment map[string]game.Piece) { swapKinds(deployment, mine, front) }},
		{name: "bomb in front row", mutate: func(deployment map[string]game.Piece) { swapKinds(deployment, bomb, front) }},
		{name: "wrong owner", mutate: func(deployment map[string]game.Piece) {
			piece := deployment[station]
			piece.Owner = game.East
			deployment[station] = piece
		}},
		{name: "missing piece", mutate: func(deployment map[string]game.Piece) { delete(deployment, station) }},
		{name: "wrong inventory count", mutate: func(deployment map[string]game.Piece) {
			deployment[station] = game.Piece{ID: "replacement", Owner: game.North, Kind: game.Flag}
		}},
		{name: "camp occupied", mutate: func(deployment map[string]game.Piece) {
			piece := deployment[station]
			delete(deployment, station)
			deployment["north-r2-2L"] = piece
		}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			deployment := cloneDeployment(base)
			tt.mutate(deployment)
			if err := game.ValidateDeployment(board, game.North, deployment); err == nil {
				t.Fatalf("invalid deployment was accepted: %#v", deployment)
			}
		})
	}
}

func TestDeploymentCannotBeReplacedAfterPlayBegins(t *testing.T) {
	board := game.NewBoard()
	state := game.NewState(game.FourDark, game.Standard, game.North, time.Now().UTC())
	state.Phase = game.Playing
	if err := state.ReplaceDeployment(board, game.North, game.DefaultDeployment(board, game.North)); err == nil {
		t.Fatal("deployment replacement was accepted during play")
	}
}
