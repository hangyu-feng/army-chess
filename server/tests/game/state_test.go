package game_test

import (
	"testing"
	"time"

	"github.com/fenghangyu/army-chess/server/internal/game"
)

func TestStateValidateRejectsMalformedPersistedState(t *testing.T) {
	now := time.Now().UTC()
	tests := []struct {
		name   string
		mutate func(*game.State)
	}{
		{name: "invalid phase", mutate: func(state *game.State) { state.Phase = "unknown" }},
		{name: "invalid visibility", mutate: func(state *game.State) { state.Mode = "unknown" }},
		{name: "invalid clock", mutate: func(state *game.State) { state.Clock = "unknown" }},
		{name: "invalid owner", mutate: func(state *game.State) {
			state.Pieces["north-r1-1L"] = game.Piece{ID: "p1", Owner: "unknown", Kind: game.Company}
		}},
		{name: "missing piece id", mutate: func(state *game.State) {
			state.Pieces["north-r1-1L"] = game.Piece{Owner: game.North, Kind: game.Company}
		}},
		{name: "missing piece kind", mutate: func(state *game.State) { state.Pieces["north-r1-1L"] = game.Piece{ID: "p1", Owner: game.North} }},
		{name: "duplicate piece id", mutate: func(state *game.State) {
			state.Pieces["north-r1-1L"] = game.Piece{ID: "same", Owner: game.North, Kind: game.Company}
			state.Pieces["north-r1-1R"] = game.Piece{ID: "same", Owner: game.North, Kind: game.Engineer}
		}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			state := game.NewState(game.FourDark, game.Standard, game.North, now)
			tt.mutate(state)
			if err := state.Validate(); err == nil {
				t.Fatal("malformed state was accepted")
			}
		})
	}
}

func TestStateCloneDoesNotShareMutableMapsOrPointers(t *testing.T) {
	now := time.Now().UTC()
	state := game.NewState(game.FourDark, game.Standard, game.North, now)
	state.Pieces["north-r1-1L"] = game.Piece{ID: "p1", Owner: game.North, Kind: game.Company}
	state.RevealedFlags[game.North] = "north-r6-2L"
	state.DrawAccepts[game.North] = true
	state.LastMove = &game.Move{Seat: game.North, From: "a", To: "b", Result: "move"}
	state.Result = &game.Result{Outcome: "draw", Reason: "test"}

	clone := state.Clone()
	clone.Pieces["north-r1-1L"] = game.Piece{ID: "p2", Owner: game.North, Kind: game.Engineer}
	clone.RevealedFlags[game.North] = "changed"
	clone.DrawAccepts[game.North] = false
	clone.LastMove.To = "changed"
	clone.Result.Reason = "changed"

	if state.Pieces["north-r1-1L"].ID != "p1" || state.RevealedFlags[game.North] != "north-r6-2L" || !state.DrawAccepts[game.North] || state.LastMove.To != "b" || state.Result.Reason != "test" {
		t.Fatalf("clone shares mutable state: original=%#v clone=%#v", state, clone)
	}
}
