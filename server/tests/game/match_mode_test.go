package game_test

import (
	"testing"
	"time"

	"github.com/fenghangyu/army-chess/server/internal/game"
)

func oneVsOneState(t *testing.T) (*game.Board, *game.State, time.Time) {
	t.Helper()
	now := time.Now().UTC()
	board := game.NewBoardForMode(game.OneVsOne)
	state := game.NewStateWithMatchMode(game.OneVsOne, game.FourDark, game.Fast, game.North, now)
	for _, seat := range state.RequiredSeats() {
		state.Players[seat] = game.Player{Username: string(seat), Ready: true}
		for node, piece := range game.DefaultDeployment(board, seat) {
			state.Pieces[node] = piece
		}
	}
	return board, state, now
}

func TestOneVsOneUsesOnlyNorthAndSouthAsOpposingTeams(t *testing.T) {
	board, state, now := oneVsOneState(t)
	if got := state.RequiredSeats(); len(got) != 2 || got[0] != game.North || got[1] != game.South {
		t.Fatalf("required seats = %v", got)
	}
	if state.Team(game.North) == state.Team(game.South) {
		t.Fatalf("1v1 seats were treated as teammates: north=%q south=%q", state.Team(game.North), state.Team(game.South))
	}
	if err := state.Start(board, now); err != nil {
		t.Fatalf("1v1 match did not start: %v", err)
	}
	state.AdvanceTurn(board, now)
	if state.Turn != game.South {
		t.Fatalf("turn did not advance to the other 1v1 player: %s", state.Turn)
	}
	state.AdvanceTurn(board, now)
	if state.Turn != game.North {
		t.Fatalf("turn did not wrap to the first 1v1 player: %s", state.Turn)
	}
}

func TestOneVsOneRequiresBothPlayersAndAwardsTheRemainingPlayer(t *testing.T) {
	board, state, now := oneVsOneState(t)
	state.Players[game.South] = game.Player{}
	if err := state.Start(board, now); err != game.ErrNotReady {
		t.Fatalf("missing 1v1 player returned %v, want %v", err, game.ErrNotReady)
	}

	_, state, now = oneVsOneState(t)
	if err := state.Start(board, now); err != nil {
		t.Fatal(err)
	}
	if err := state.Resign(board, game.South, now); err != nil {
		t.Fatalf("south resignation failed: %v", err)
	}
	if state.Phase != game.Finished || state.Result == nil || state.Result.Team != state.Team(game.North) {
		t.Fatalf("1v1 resignation did not award north: phase=%s result=%#v", state.Phase, state.Result)
	}
}

func TestOneVsOneRejectsUnavailableSeatReadyState(t *testing.T) {
	board, state, now := oneVsOneState(t)
	if err := state.SetReady(board, game.East, true, now); err == nil {
		t.Fatal("east was allowed to ready in 1v1")
	}
	if state.SeatEnabled(game.East) || state.SeatEnabled(game.West) {
		t.Fatal("inactive 1v1 seats were reported as enabled")
	}
}
