package game

import (
	"strings"
	"testing"
	"time"
)

func TestDefaultMatchCanStartAndAdvance(t *testing.T) {
	board := NewBoard()
	now := time.Now().UTC()
	state := NewState(FourDark, Fast, North, now)
	for _, seat := range Seats {
		state.Players[seat] = Player{Username: string(seat), Ready: true}
		for node, piece := range DefaultDeployment(board, seat) {
			state.Pieces[node] = piece
		}
	}
	if err := state.Start(board, now); err != nil {
		t.Fatal(err)
	}
	if state.Phase != Playing || state.Turn != North || state.Deadline.IsZero() {
		t.Fatalf("unexpected started state: %#v", state)
	}
	legal := state.LegalMoves(board, North)
	if len(legal) == 0 {
		t.Fatal("opening seat should have a legal move")
	}
	parts := strings.Split(legal[0], "->")
	if _, err := state.Move(board, North, parts[0], parts[1], now); err != nil {
		t.Fatalf("opening move rejected: %v", err)
	}
	if state.Turn != East {
		t.Fatalf("turn did not advance clockwise: %s", state.Turn)
	}
}

func TestFlagCaptureEliminatesPlayerAndRevealsLocation(t *testing.T) {
	board := NewBoard()
	now := time.Now().UTC()
	state := NewState(FourDark, Fast, North, now)
	state.Phase = Playing
	state.Turn = North
	for _, seat := range Seats {
		state.Players[seat] = Player{Username: string(seat)}
	}
	state.Pieces["n04_03"] = Piece{ID: "north-attacker", Owner: North, Kind: Commander}
	state.Pieces["n05_03"] = Piece{ID: "east-flag", Owner: East, Kind: Flag}
	if _, err := state.Move(board, North, "n04_03", "n05_03", now); err != nil {
		t.Fatal(err)
	}
	if !state.Players[East].Eliminated || state.RevealedFlags[East] != "n05_03" {
		t.Fatalf("flag capture did not eliminate/reveal: %#v %v", state.Players[East], state.RevealedFlags)
	}
	if state.Pieces["n05_03"].Owner != North {
		t.Fatalf("winning piece did not occupy captured flag position: %#v, phase=%s turn=%s", state.Pieces, state.Phase, state.Turn)
	}
}

func TestCommanderRemovalRevealsTheOwnerFlag(t *testing.T) {
	board := NewBoard()
	now := time.Now().UTC()
	state := NewState(FourDark, Fast, North, now)
	state.Phase = Playing
	state.Turn = North
	for _, seat := range Seats {
		state.Players[seat] = Player{Username: string(seat)}
	}
	state.Pieces["n00_00"] = Piece{ID: "north-flag", Owner: North, Kind: Flag}
	state.Pieces["n04_03"] = Piece{ID: "north-commander", Owner: North, Kind: Commander}
	state.Pieces["n05_03"] = Piece{ID: "east-bomb", Owner: East, Kind: Bomb}
	if _, err := state.Move(board, North, "n04_03", "n05_03", now); err != nil {
		t.Fatal(err)
	}
	if state.RevealedFlags[North] != "n00_00" {
		t.Fatalf("commander removal did not reveal the flag: %#v", state.RevealedFlags)
	}
}

func TestSetupCannotBeginBeforeAllSeatsAndDeadlineSubmitsValidLayouts(t *testing.T) {
	board := NewBoard()
	now := time.Now().UTC()
	state := NewState(FourDark, Fast, North, now)
	state.Players[North] = Player{Username: "north"}
	if err := state.SetReady(board, North, true, now); err != ErrNotReady {
		t.Fatalf("expected all-seat guard, got %v", err)
	}
	if state.Phase != Lobby {
		t.Fatalf("phase changed before all seats joined: %s", state.Phase)
	}
	for _, seat := range Seats {
		state.Players[seat] = Player{Username: string(seat)}
		for node, piece := range DefaultDeployment(board, seat) {
			state.Pieces[node] = piece
		}
	}
	if err := state.SetReady(board, North, true, now); err != nil {
		t.Fatal(err)
	}
	if state.Phase != Setup {
		t.Fatalf("expected setup phase, got %s", state.Phase)
	}
	if !state.Tick(board, state.SetupDeadline.Add(time.Second)) || state.Phase != Playing {
		t.Fatalf("setup deadline did not start valid match: phase=%s", state.Phase)
	}
}
