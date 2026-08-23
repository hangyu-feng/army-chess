package game_test

import (
	"strings"
	"testing"
	"time"

	"github.com/fenghangyu/army-chess/server/internal/game"
)

func TestDefaultMatchCanStartAndAdvance(t *testing.T) {
	board := game.NewBoard()
	now := time.Now().UTC()
	state := game.NewState(game.FourDark, game.Fast, game.North, now)
	for _, seat := range game.Seats {
		state.Players[seat] = game.Player{Username: string(seat), Ready: true}
		for node, piece := range game.DefaultDeployment(board, seat) {
			state.Pieces[node] = piece
		}
	}
	if err := state.Start(board, now); err != nil {
		t.Fatal(err)
	}
	if state.Phase != game.Playing || state.Turn != game.North || state.Deadline.IsZero() {
		t.Fatalf("unexpected started state: %#v", state)
	}
	legal := state.LegalMoves(board, game.North)
	if len(legal) == 0 {
		t.Fatal("opening seat should have a legal move")
	}
	parts := strings.Split(legal[0], "->")
	if _, err := state.Move(board, game.North, parts[0], parts[1], now); err != nil {
		t.Fatalf("opening move rejected: %v", err)
	}
	if state.Turn != game.East {
		t.Fatalf("turn did not advance clockwise: %s", state.Turn)
	}
}

func TestFlagCaptureEliminatesPlayerAndRevealsLocation(t *testing.T) {
	board := game.NewBoard()
	now := time.Now().UTC()
	state := game.NewState(game.FourDark, game.Fast, game.North, now)
	state.Phase = game.Playing
	state.Turn = game.North
	for _, seat := range game.Seats {
		state.Players[seat] = game.Player{Username: string(seat)}
	}
	state.Pieces["north-r2-1L"] = game.Piece{ID: "north-attacker", Owner: game.North, Kind: game.Commander}
	state.Pieces["north-r3-1L"] = game.Piece{ID: "east-flag", Owner: game.East, Kind: game.Flag}
	if _, err := state.Move(board, game.North, "north-r2-1L", "north-r3-1L", now); err != nil {
		t.Fatal(err)
	}
	if !state.Players[game.East].Eliminated || state.RevealedFlags[game.East] != "north-r3-1L" {
		t.Fatalf("flag capture did not eliminate/reveal: %#v %v", state.Players[game.East], state.RevealedFlags)
	}
	if state.Pieces["north-r3-1L"].Owner != game.North {
		t.Fatalf("winning piece did not occupy captured flag position: %#v, phase=%s turn=%s", state.Pieces, state.Phase, state.Turn)
	}
}

func TestCommanderRemovalRevealsTheOwnerFlag(t *testing.T) {
	board := game.NewBoard()
	now := time.Now().UTC()
	state := game.NewState(game.FourDark, game.Fast, game.North, now)
	state.Phase = game.Playing
	state.Turn = game.North
	for _, seat := range game.Seats {
		state.Players[seat] = game.Player{Username: string(seat)}
	}
	state.Pieces["north-r6-2L"] = game.Piece{ID: "north-flag", Owner: game.North, Kind: game.Flag}
	state.Pieces["north-r2-1L"] = game.Piece{ID: "north-commander", Owner: game.North, Kind: game.Commander}
	state.Pieces["north-r3-1L"] = game.Piece{ID: "east-bomb", Owner: game.East, Kind: game.Bomb}
	if _, err := state.Move(board, game.North, "north-r2-1L", "north-r3-1L", now); err != nil {
		t.Fatal(err)
	}
	if state.RevealedFlags[game.North] != "north-r6-2L" {
		t.Fatalf("commander removal did not reveal the flag: %#v", state.RevealedFlags)
	}
}

func TestSetupCannotBeginBeforeAllSeatsAndDeadlineSubmitsValidLayouts(t *testing.T) {
	board := game.NewBoard()
	now := time.Now().UTC()
	state := game.NewState(game.FourDark, game.Fast, game.North, now)
	state.Players[game.North] = game.Player{Username: "north"}
	if err := state.SetReady(board, game.North, true, now); err != game.ErrNotReady {
		t.Fatalf("expected all-seat guard, got %v", err)
	}
	if state.Phase != game.Lobby {
		t.Fatalf("phase changed before all seats joined: %s", state.Phase)
	}
	for _, seat := range game.Seats {
		state.Players[seat] = game.Player{Username: string(seat)}
		for node, piece := range game.DefaultDeployment(board, seat) {
			state.Pieces[node] = piece
		}
	}
	if err := state.SetReady(board, game.North, true, now); err != nil {
		t.Fatal(err)
	}
	if state.Phase != game.Setup {
		t.Fatalf("expected setup phase, got %s", state.Phase)
	}
	if !state.Tick(board, state.SetupDeadline.Add(time.Second)) || state.Phase != game.Playing {
		t.Fatalf("setup deadline did not start valid match: phase=%s", state.Phase)
	}
}
