package game

import (
	"testing"
	"time"
)

func TestDefaultDeploymentIsValidForEverySeat(t *testing.T) {
	board := NewBoard()
	for _, seat := range Seats {
		deployment := DefaultDeployment(board, seat)
		if err := ValidateDeployment(board, seat, deployment); err != nil {
			t.Fatalf("%s default deployment: %v", seat, err)
		}
		if len(deployment) != 25 {
			t.Fatalf("%s has %d deployment positions", seat, len(deployment))
		}
	}
}

func TestStateProjectionHidesRanksByMode(t *testing.T) {
	board := NewBoard()
	state := NewState(FourDark, Standard, North, time.Now())
	for _, seat := range Seats {
		for node, piece := range DefaultDeployment(board, seat) {
			state.Pieces[node] = piece
		}
		state.Players[seat] = Player{Username: string(seat)}
	}
	own := state.Project(Viewer{Seat: North}, board)
	if own.Pieces["n00_00"].Kind != Flag {
		t.Fatalf("owner should see their own rank")
	}
	if own.Pieces["n07_00"].Kind != "" {
		t.Fatalf("four-dark player saw an opponent rank")
	}
	team := state
	team.Mode = DoubleVisible
	teamView := team.Project(Viewer{Seat: South}, board)
	if teamView.Pieces["n00_00"].Kind != Flag {
		t.Fatalf("teammate rank should be visible")
	}
	spectator := state.Project(Viewer{Spectator: true}, board)
	for node, piece := range spectator.Pieces {
		if piece.Kind != "" {
			t.Fatalf("spectator saw %s at %s", piece.Kind, node)
		}
	}
}

func TestCombatMatrixSpecialPieces(t *testing.T) {
	attacker := Piece{Owner: North, Kind: Engineer}
	mine := Piece{Owner: East, Kind: Mine}
	winner, result := resolveCombat(attacker, mine)
	if winner == nil || winner.Kind != Engineer || result != "engineer_cleared_mine" {
		t.Fatalf("engineer versus mine = %#v, %s", winner, result)
	}
	winner, result = resolveCombat(Piece{Owner: North, Kind: Company}, mine)
	if winner == nil || winner.Kind != Mine || result != "mine_survives" {
		t.Fatalf("company versus mine = %#v, %s", winner, result)
	}
	winner, result = resolveCombat(Piece{Owner: North, Kind: Marshal}, Piece{Owner: East, Kind: Marshal})
	if winner != nil || result != "both_removed" {
		t.Fatalf("equal ranks = %#v, %s", winner, result)
	}
}
