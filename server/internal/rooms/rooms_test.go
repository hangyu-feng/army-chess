package rooms

import (
	"context"
	"encoding/json"
	"log/slog"
	"strings"
	"testing"
	"time"

	"github.com/fenghangyu/army-chess/server/internal/game"
)

func TestRoomLifecycleStartsMatchAndAcceptsOpeningMove(t *testing.T) {
	registry := NewRegistry(slog.Default(), nil)
	room, err := registry.Create(context.Background(), "p-north", "north_user", "")
	if err != nil {
		t.Fatal(err)
	}
	for _, participant := range []struct {
		id, username string
	}{
		{"p-east", "east_user"}, {"p-south", "south_user"}, {"p-west", "west_user"},
	} {
		if err := room.Join(context.Background(), participant.id, participant.username, "", false); err != nil {
			t.Fatal(err)
		}
	}
	now := time.Now().UTC()
	for index, seat := range game.Seats {
		id := []string{"p-north", "p-east", "p-south", "p-west"}[index]
		payload, _ := json.Marshal(map[string]any{"seat": seat})
		if err := room.Handle(id, Envelope{Type: "seat.select", RequestID: "seat-" + string(seat), Payload: payload}, now); err != nil {
			t.Fatalf("select %s: %v", seat, err)
		}
	}
	for index, seat := range game.Seats {
		id := []string{"p-north", "p-east", "p-south", "p-west"}[index]
		payload, _ := json.Marshal(map[string]any{"ready": true})
		if err := room.Handle(id, Envelope{Type: "ready", RequestID: "ready-" + string(seat), Payload: payload}, now); err != nil {
			t.Fatalf("ready %s: %v", seat, err)
		}
	}
	if room.State.Phase != game.Playing {
		t.Fatalf("room did not start: %s", room.State.Phase)
	}
	legal := room.State.LegalMoves(room.Board, room.State.Turn)
	if len(legal) == 0 {
		t.Fatal("opening player has no legal move")
	}
	parts := strings.Split(legal[0], "->")
	payload, _ := json.Marshal(map[string]string{"from": parts[0], "to": parts[1]})
	turnID := map[game.Seat]string{game.North: "p-north", game.East: "p-east", game.South: "p-south", game.West: "p-west"}[room.State.Turn]
	if err := room.Handle(turnID, Envelope{Type: "move", RequestID: "move-1", Payload: payload}, now); err != nil {
		t.Fatalf("opening move: %v", err)
	}
	if room.State.Version < 2 || room.State.Turn == room.State.Opening {
		t.Fatalf("move did not advance the room: version=%d turn=%s", room.State.Version, room.State.Turn)
	}
}

func TestSpectatorProjectionNeverContainsRanks(t *testing.T) {
	registry := NewRegistry(slog.Default(), nil)
	room, err := registry.Create(context.Background(), "host", "host_user", "")
	if err != nil {
		t.Fatal(err)
	}
	if err := room.Join(context.Background(), "spectator", "spectator_user", "", true); err != nil {
		t.Fatal(err)
	}
	if err := room.Handle("host", Envelope{Type: "seat.select", Payload: json.RawMessage(`{"seat":"north"}`)}, time.Now().UTC()); err != nil {
		t.Fatal(err)
	}
	view, err := room.ViewFor("spectator")
	if err != nil {
		t.Fatal(err)
	}
	for node, piece := range view.Pieces {
		if piece.Kind != "" {
			t.Fatalf("spectator saw %s at %s", piece.Kind, node)
		}
	}
}

func TestRematchClearsSeatsAfterAllPlayersAgree(t *testing.T) {
	registry := NewRegistry(slog.Default(), nil)
	room, err := registry.Create(context.Background(), "p-north", "north_user", "")
	if err != nil {
		t.Fatal(err)
	}
	participants := []struct {
		id, username string
	}{
		{"p-east", "east_user"}, {"p-south", "south_user"}, {"p-west", "west_user"},
	}
	for _, participant := range participants {
		if err := room.Join(context.Background(), participant.id, participant.username, "", false); err != nil {
			t.Fatal(err)
		}
	}
	for index, seat := range game.Seats {
		id := []string{"p-north", "p-east", "p-south", "p-west"}[index]
		payload, _ := json.Marshal(map[string]string{"seat": string(seat)})
		if err := room.Handle(id, Envelope{Type: "seat.select", Payload: payload}, time.Now().UTC()); err != nil {
			t.Fatal(err)
		}
	}
	room.State.Phase = game.Finished
	room.State.Result = &game.Result{Outcome: "draw", Reason: "test"}
	for index := range game.Seats {
		id := []string{"p-north", "p-east", "p-south", "p-west"}[index]
		payload := json.RawMessage(`{"ready":true}`)
		if err := room.Handle(id, Envelope{Type: "rematch.ready", Payload: payload}, time.Now().UTC()); err != nil {
			t.Fatal(err)
		}
	}
	if room.State.Phase != game.Lobby || len(room.State.Pieces) != 0 {
		t.Fatalf("rematch did not reset lobby: phase=%s pieces=%d", room.State.Phase, len(room.State.Pieces))
	}
	for id, participant := range room.Participants {
		if participant.Seat.Valid() {
			t.Fatalf("participant %s retained seat %s", id, participant.Seat)
		}
	}
}
