package game_test

import (
	"testing"
	"time"

	"github.com/fenghangyu/army-chess/server/internal/game"
)

func playingState(t *testing.T, turn game.Seat) (*game.Board, *game.State, time.Time) {
	t.Helper()
	now := time.Now().UTC()
	board := game.NewBoard()
	state := game.NewState(game.FourDark, game.Fast, game.North, now)
	state.Phase = game.Playing
	state.Turn = turn
	for _, seat := range game.Seats {
		state.Players[seat] = game.Player{Username: string(seat)}
	}
	return board, state, now
}

func addMobilePiece(state *game.State, node string, owner game.Seat, kind game.PieceKind) {
	state.Pieces[node] = game.Piece{ID: string(owner) + "-" + node, Owner: owner, Kind: kind}
}

func TestCombatCoversRanksBombsMinesAndFlags(t *testing.T) {
	tests := []struct {
		name        string
		attacker    game.PieceKind
		defender    game.PieceKind
		wantResult  string
		wantKind    game.PieceKind
		wantPresent bool
		captureFlag bool
	}{
		{name: "higher rank survives", attacker: game.Commander, defender: game.Marshal, wantResult: "attacker_survives", wantKind: game.Commander, wantPresent: true},
		{name: "lower rank loses", attacker: game.Engineer, defender: game.Commander, wantResult: "defender_survives", wantKind: game.Commander, wantPresent: true},
		{name: "equal ranks trade", attacker: game.Marshal, defender: game.Marshal, wantResult: "both_removed", wantPresent: false},
		{name: "engineer clears mine", attacker: game.Engineer, defender: game.Mine, wantResult: "engineer_cleared_mine", wantKind: game.Engineer, wantPresent: true},
		{name: "ordinary piece loses to mine", attacker: game.Company, defender: game.Mine, wantResult: "mine_survives", wantKind: game.Mine, wantPresent: true},
		{name: "bomb destroys both", attacker: game.Bomb, defender: game.Company, wantResult: "both_removed", wantPresent: false},
		{name: "bomb defender destroys both", attacker: game.Company, defender: game.Bomb, wantResult: "both_removed", wantPresent: false},
		{name: "flag capture eliminates owner", attacker: game.Commander, defender: game.Flag, wantResult: "attacker_survives", wantKind: game.Commander, wantPresent: true, captureFlag: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			board, state, now := playingState(t, game.North)
			attacker := game.Piece{ID: "attacker", Owner: game.North, Kind: tt.attacker}
			defenderOwner := game.East
			if tt.captureFlag {
				state.Pieces["east-r6-2L"] = game.Piece{ID: "east-flag", Owner: game.East, Kind: game.Flag}
			}
			state.Pieces["north-r2-1L"] = attacker
			state.Pieces["north-r3-1L"] = game.Piece{ID: "defender", Owner: defenderOwner, Kind: tt.defender}
			// Keep the other countries playable so post-combat turn advancement
			// does not obscure the combat result being tested.
			addMobilePiece(state, "east-r1-3", game.East, game.Engineer)
			addMobilePiece(state, "south-r1-3", game.South, game.Engineer)
			addMobilePiece(state, "west-r1-3", game.West, game.Engineer)

			result, err := state.Move(board, game.North, "north-r2-1L", "north-r3-1L", now)
			if err != nil {
				t.Fatal(err)
			}
			if result != tt.wantResult {
				t.Fatalf("combat result = %q, want %q", result, tt.wantResult)
			}
			piece, present := state.Pieces["north-r3-1L"]
			if present != tt.wantPresent || (present && piece.Kind != tt.wantKind) {
				t.Fatalf("destination = %#v, present=%t", piece, present)
			}
			if tt.captureFlag {
				if !state.Players[game.East].Eliminated || state.RevealedFlags[game.East] != "north-r3-1L" {
					t.Fatalf("flag owner was not eliminated/revealed: player=%#v flags=%#v", state.Players[game.East], state.RevealedFlags)
				}
				if _, present := state.Pieces["east-r6-2L"]; present {
					t.Fatal("captured player's remaining flag was not removed")
				}
			}
		})
	}
}

func TestCommanderRemovalRevealsFlagAfterAnyCombat(t *testing.T) {
	board, state, now := playingState(t, game.North)
	state.Pieces["north-r6-2L"] = game.Piece{ID: "north-flag", Owner: game.North, Kind: game.Flag}
	state.Pieces["north-r2-1L"] = game.Piece{ID: "north-commander", Owner: game.North, Kind: game.Commander}
	state.Pieces["north-r3-1L"] = game.Piece{ID: "east-bomb", Owner: game.East, Kind: game.Bomb}
	addMobilePiece(state, "east-r1-3", game.East, game.Engineer)
	addMobilePiece(state, "south-r1-3", game.South, game.Engineer)
	addMobilePiece(state, "west-r1-3", game.West, game.Engineer)

	if _, err := state.Move(board, game.North, "north-r2-1L", "north-r3-1L", now); err != nil {
		t.Fatal(err)
	}
	if got := state.RevealedFlags[game.North]; got != "north-r6-2L" {
		t.Fatalf("commander removal did not reveal flag: %q", got)
	}
}

func TestAdvanceTurnSkipsEliminatedAndNoMoveSeats(t *testing.T) {
	board, state, now := playingState(t, game.North)
	addMobilePiece(state, "north-r2-3", game.North, game.Company)
	state.Pieces["east-r6-2L"] = game.Piece{ID: "east-flag", Owner: game.East, Kind: game.Flag}
	addMobilePiece(state, "south-r2-3", game.South, game.Company)
	addMobilePiece(state, "west-r2-3", game.West, game.Company)

	state.AdvanceTurn(board, now)
	if !state.Players[game.East].Eliminated || state.Players[game.East].EliminationReason != "no_legal_move" {
		t.Fatalf("no-move seat was not eliminated: %#v", state.Players[game.East])
	}
	if state.Turn != game.South {
		t.Fatalf("turn did not skip eliminated east seat: %s", state.Turn)
	}
}

func TestFiveMissedDeadlinesEliminateTheCurrentSeat(t *testing.T) {
	board, state, now := playingState(t, game.North)
	for _, seat := range game.Seats {
		addMobilePiece(state, string(seat)+"-r2-3", seat, game.Company)
	}
	for miss := 0; miss < 5; miss++ {
		state.Turn = game.North
		state.Deadline = now.Add(-time.Second)
		if !state.Tick(board, now) {
			t.Fatalf("expired deadline %d was not processed", miss+1)
		}
	}
	if !state.Players[game.North].Eliminated || state.Players[game.North].Misses != 5 || state.Players[game.North].EliminationReason != "missed_deadlines" {
		t.Fatalf("missed deadline elimination: %#v", state.Players[game.North])
	}
	if state.Phase != game.Playing || state.Turn != game.East {
		t.Fatalf("unexpected state after timeout elimination: phase=%s turn=%s", state.Phase, state.Turn)
	}
}

func TestSeventyNonCapturingMovesEndInADraw(t *testing.T) {
	board, state, now := playingState(t, game.North)
	state.NoCaptureMoves = 69
	addMobilePiece(state, "north-r2-3", game.North, game.Company)
	addMobilePiece(state, "east-r2-3", game.East, game.Company)
	addMobilePiece(state, "south-r2-3", game.South, game.Company)
	addMobilePiece(state, "west-r2-3", game.West, game.Company)

	result, err := state.Move(board, game.North, "north-r2-3", "north-r3-3", now)
	if err != nil {
		t.Fatal(err)
	}
	if result != "move" || state.Phase != game.Finished || state.Result == nil || state.Result.Reason != "seventy_moves" || state.Result.Outcome != "draw" {
		t.Fatalf("seventy-move result: result=%q phase=%s stateResult=%#v", result, state.Phase, state.Result)
	}
}

func TestDrawOfferCanBeRejectedOrAcceptedUnanimously(t *testing.T) {
	_, state, _ := playingState(t, game.North)
	if err := state.OfferDraw(game.North); err != nil {
		t.Fatal(err)
	}
	if err := state.OfferDraw(game.East); err == nil {
		t.Fatal("second draw offer was accepted")
	}
	if err := state.RespondDraw(game.East, false); err != nil {
		t.Fatal(err)
	}
	if state.DrawOffer != "" || len(state.DrawAccepts) != 0 {
		t.Fatalf("rejected offer was not cleared: offer=%s accepts=%#v", state.DrawOffer, state.DrawAccepts)
	}
	if err := state.OfferDraw(game.South); err != nil {
		t.Fatal(err)
	}
	for _, seat := range []game.Seat{game.North, game.East, game.West} {
		if err := state.RespondDraw(seat, true); err != nil {
			t.Fatalf("draw acceptance from %s: %v", seat, err)
		}
	}
	if state.Phase != game.Finished || state.Result == nil || state.Result.Outcome != "draw" || state.Result.Reason != "unanimous_offer" {
		t.Fatalf("unanimous draw did not finish match: phase=%s result=%#v", state.Phase, state.Result)
	}
}

func TestResigningBothTeammatesWinsForTheOtherTeam(t *testing.T) {
	board, state, now := playingState(t, game.North)
	for _, seat := range game.Seats {
		addMobilePiece(state, string(seat)+"-r2-3", seat, game.Company)
	}
	if err := state.Resign(board, game.North, now); err != nil {
		t.Fatal(err)
	}
	if state.Phase != game.Playing {
		t.Fatalf("one resignation ended match: %#v", state.Result)
	}
	if err := state.Resign(board, game.South, now); err != nil {
		t.Fatal(err)
	}
	if state.Phase != game.Finished || state.Result == nil || state.Result.Outcome != "win" || state.Result.Team != "east_west" || state.Result.Reason != "east_west" {
		t.Fatalf("teammate resignations did not award other team: phase=%s result=%#v", state.Phase, state.Result)
	}
}
