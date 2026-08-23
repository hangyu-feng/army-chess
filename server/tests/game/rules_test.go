package game_test

import (
	"testing"
	"time"

	"github.com/fenghangyu/army-chess/server/internal/game"
)

func TestDefaultDeploymentIsValidForEverySeat(t *testing.T) {
	board := game.NewBoard()
	for _, seat := range game.Seats {
		deployment := game.DefaultDeployment(board, seat)
		if err := game.ValidateDeployment(board, seat, deployment); err != nil {
			t.Fatalf("%s default deployment: %v", seat, err)
		}
		if len(deployment) != 25 {
			t.Fatalf("%s has %d deployment positions", seat, len(deployment))
		}
	}
}

func TestRailwayTurningRulesDistinguishEngineers(t *testing.T) {
	board := game.NewBoard()
	for _, kind := range []game.PieceKind{game.Company, game.Engineer} {
		state := game.NewState(game.FourDark, game.Fast, game.North, time.Now())
		state.Phase = game.Playing
		state.Turn = game.North
		for _, seat := range game.Seats {
			state.Players[seat] = game.Player{Username: string(seat)}
		}
		state.Pieces["north-r1-2L"] = game.Piece{Owner: game.North, Kind: kind}
		moves := state.LegalMoves(board, game.North)
		turningRoute := "north-r1-2L->north-r2-1L"
		found := false
		for _, move := range moves {
			if move == turningRoute {
				found = true
				break
			}
		}
		if kind == game.Engineer && !found {
			t.Fatalf("engineer should be able to turn on railway: %v", moves)
		}
		if kind != game.Engineer && found {
			t.Fatalf("ordinary piece should not turn on railway: %v", moves)
		}
	}
}

func TestRoadsAreSingleEdgeAndOccupiedCampsAreSafe(t *testing.T) {
	board := game.NewBoard()
	state := game.NewState(game.FourDark, game.Fast, game.North, time.Now())
	state.Phase = game.Playing
	state.Turn = game.North
	for _, seat := range game.Seats {
		state.Players[seat] = game.Player{Username: string(seat)}
	}
	state.Pieces["north-r2-3"] = game.Piece{Owner: game.North, Kind: game.Company}
	moves := state.LegalMoves(board, game.North)
	contains := func(want string) bool {
		for _, move := range moves {
			if move == want {
				return true
			}
		}
		return false
	}
	if !contains("north-r2-3->north-r3-3") {
		t.Fatalf("piece should be able to enter an empty camp: %v", moves)
	}
	if contains("north-r2-3->north-r4-3") {
		t.Fatalf("road movement must stop after one edge: %v", moves)
	}
	state.Pieces["north-r3-3"] = game.Piece{Owner: game.East, Kind: game.Company}
	for _, move := range state.LegalMoves(board, game.North) {
		if move == "north-r2-3->north-r3-3" {
			t.Fatal("occupied camp must not be attacked")
		}
	}
}

func TestStateProjectionHidesRanksByMode(t *testing.T) {
	board := game.NewBoard()
	state := game.NewState(game.FourDark, game.Standard, game.North, time.Now())
	for _, seat := range game.Seats {
		for node, piece := range game.DefaultDeployment(board, seat) {
			state.Pieces[node] = piece
		}
		state.Players[seat] = game.Player{Username: string(seat)}
	}
	flagNode := ""
	for node, piece := range state.Pieces {
		if piece.Owner == game.North && piece.Kind == game.Flag {
			flagNode = node
			break
		}
	}
	if flagNode == "" {
		t.Fatal("default deployment did not place a north flag")
	}
	own := state.Project(game.Viewer{Seat: game.North}, board)
	if own.Pieces[flagNode].Kind != game.Flag {
		t.Fatalf("owner should see their own rank")
	}
	opponentNode := ""
	for node, piece := range state.Pieces {
		if piece.Owner == game.East {
			opponentNode = node
			break
		}
	}
	if own.Pieces[opponentNode].Kind != "" {
		t.Fatalf("four-dark player saw an opponent rank")
	}
	team := state
	team.Mode = game.DoubleVisible
	teamView := team.Project(game.Viewer{Seat: game.South}, board)
	if teamView.Pieces[flagNode].Kind != game.Flag {
		t.Fatalf("teammate rank should be visible")
	}
	spectator := state.Project(game.Viewer{Spectator: true}, board)
	for node, piece := range spectator.Pieces {
		if piece.Kind != "" {
			t.Fatalf("spectator saw %s at %s", piece.Kind, node)
		}
	}
}

func TestRevealedFlagIsVisibleToEveryViewer(t *testing.T) {
	board := game.NewBoard()
	state := game.NewState(game.FourDark, game.Fast, game.North, time.Now())
	state.Pieces["north-r6-2L"] = game.Piece{ID: "north-flag", Owner: game.North, Kind: game.Flag}
	state.RevealedFlags[game.North] = "north-r6-2L"
	for _, viewer := range []game.Viewer{{Seat: game.East}, {Seat: game.South}, {Spectator: true}} {
		view := state.Project(viewer, board)
		piece := view.Pieces["north-r6-2L"]
		if piece.Kind != game.Flag || !piece.Revealed {
			t.Fatalf("revealed flag hidden from %#v: %#v", viewer, piece)
		}
	}
}

func TestCombatMatrixSpecialPieces(t *testing.T) {
	tests := []struct {
		name           string
		attacker       game.Piece
		defender       game.Piece
		expectedResult string
		expectedKind   game.PieceKind
		expectedPiece  bool
	}{
		{
			name:           "engineer clears mine",
			attacker:       game.Piece{Owner: game.North, Kind: game.Engineer},
			defender:       game.Piece{Owner: game.East, Kind: game.Mine},
			expectedResult: "engineer_cleared_mine",
			expectedKind:   game.Engineer,
			expectedPiece:  true,
		},
		{
			name:           "mine survives non-engineer",
			attacker:       game.Piece{Owner: game.North, Kind: game.Company},
			defender:       game.Piece{Owner: game.East, Kind: game.Mine},
			expectedResult: "mine_survives",
			expectedKind:   game.Mine,
			expectedPiece:  true,
		},
		{
			name:           "equal ranks are removed",
			attacker:       game.Piece{Owner: game.North, Kind: game.Marshal},
			defender:       game.Piece{Owner: game.East, Kind: game.Marshal},
			expectedResult: "both_removed",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			board := game.NewBoard()
			now := time.Now().UTC()
			state := game.NewState(game.FourDark, game.Fast, game.North, now)
			state.Phase = game.Playing
			state.Turn = game.North
			for _, seat := range game.Seats {
				state.Players[seat] = game.Player{Username: string(seat)}
			}
			state.Pieces["north-r2-1L"] = tt.attacker
			state.Pieces["north-r3-1L"] = tt.defender
			// Keep East active after combat so the public Move API does not
			// remove a surviving mine merely because it has no legal move.
			state.Pieces["east-r1-3"] = game.Piece{Owner: game.East, Kind: game.Engineer}

			result, err := state.Move(board, game.North, "north-r2-1L", "north-r3-1L", now)
			if err != nil {
				t.Fatal(err)
			}
			if result != tt.expectedResult {
				t.Fatalf("combat result = %q, want %q", result, tt.expectedResult)
			}
			piece, exists := state.Pieces["north-r3-1L"]
			if exists != tt.expectedPiece || (exists && piece.Kind != tt.expectedKind) {
				t.Fatalf("destination after combat = %#v, exists=%t", piece, exists)
			}
		})
	}
}
