package rooms_test

import (
	"context"
	"encoding/json"
	"log/slog"
	"strings"
	"testing"
	"time"

	"github.com/fenghangyu/army-chess/server/internal/game"
	"github.com/fenghangyu/army-chess/server/internal/rooms"
)

func TestRoomLifecycleStartsMatchAndAcceptsOpeningMove(t *testing.T) {
	registry := rooms.NewRegistry(slog.Default(), nil)
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
		if err := room.Handle(id, rooms.Envelope{Type: "seat.select", RequestID: "seat-" + string(seat), Payload: payload}, now); err != nil {
			t.Fatalf("select %s: %v", seat, err)
		}
	}
	for index, seat := range game.Seats {
		id := []string{"p-north", "p-east", "p-south", "p-west"}[index]
		payload, _ := json.Marshal(map[string]any{"ready": true})
		if err := room.Handle(id, rooms.Envelope{Type: "ready", RequestID: "ready-" + string(seat), Payload: payload}, now); err != nil {
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
	if err := room.Handle(turnID, rooms.Envelope{Type: "move", RequestID: "move-1", Payload: payload}, now); err != nil {
		t.Fatalf("opening move: %v", err)
	}
	if room.State.Version < 2 || room.State.Turn == room.State.Opening {
		t.Fatalf("move did not advance the room: version=%d turn=%s", room.State.Version, room.State.Turn)
	}
}

func TestSpectatorProjectionNeverContainsRanks(t *testing.T) {
	registry := rooms.NewRegistry(slog.Default(), nil)
	room, err := registry.Create(context.Background(), "host", "host_user", "")
	if err != nil {
		t.Fatal(err)
	}
	if err := room.Join(context.Background(), "spectator", "spectator_user", "", true); err != nil {
		t.Fatal(err)
	}
	if err := room.Handle("host", rooms.Envelope{Type: "seat.select", Payload: json.RawMessage(`{"seat":"north"}`)}, time.Now().UTC()); err != nil {
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

func TestPlayerCanArrangePiecesWhileRoomIsWaitingForPlayers(t *testing.T) {
	registry := rooms.NewRegistry(slog.Default(), nil)
	room, err := registry.Create(context.Background(), "host", "host_user", "")
	if err != nil {
		t.Fatal(err)
	}
	if err := room.Handle("host", rooms.Envelope{Type: "seat.select", Payload: json.RawMessage(`{"seat":"north"}`)}, time.Now().UTC()); err != nil {
		t.Fatal(err)
	}
	nodes := room.Board.DeploymentNodes(game.North)
	if len(nodes) < 2 {
		t.Fatal("north deployment has fewer than two positions")
	}
	pieces := map[string]game.Piece{}
	for node, piece := range room.State.Pieces {
		if piece.Owner == game.North {
			pieces[node] = piece
		}
	}
	first, second := pieces[nodes[0]], pieces[nodes[1]]
	expectedFirst, expectedSecond := second.Kind, first.Kind
	first.Kind, second.Kind = second.Kind, first.Kind
	first.ID, second.ID = second.ID, first.ID
	pieces[nodes[0]], pieces[nodes[1]] = first, second
	payload, _ := json.Marshal(map[string]any{"pieces": pieces})
	if err := room.Handle("host", rooms.Envelope{Type: "setup.replace", Payload: payload}, time.Now().UTC()); err != nil {
		t.Fatalf("lobby deployment edit rejected: %v", err)
	}
	if room.State.Phase != game.Lobby || room.State.Pieces[nodes[0]].Kind != expectedFirst || room.State.Pieces[nodes[1]].Kind != expectedSecond {
		t.Fatalf("deployment edit was not retained: phase=%s pieces=%#v", room.State.Phase, room.State.Pieces)
	}
}

func TestRematchClearsSeatsAfterAllPlayersAgree(t *testing.T) {
	registry := rooms.NewRegistry(slog.Default(), nil)
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
		if err := room.Handle(id, rooms.Envelope{Type: "seat.select", Payload: payload}, time.Now().UTC()); err != nil {
			t.Fatal(err)
		}
	}
	room.State.Phase = game.Finished
	room.State.Result = &game.Result{Outcome: "draw", Reason: "test"}
	for index := range game.Seats {
		id := []string{"p-north", "p-east", "p-south", "p-west"}[index]
		payload := json.RawMessage(`{"ready":true}`)
		if err := room.Handle(id, rooms.Envelope{Type: "rematch.ready", Payload: payload}, time.Now().UTC()); err != nil {
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

func TestRoomRejectsInvalidOrOccupiedSeatSelections(t *testing.T) {
	registry := rooms.NewRegistry(slog.Default(), nil)
	room, err := registry.Create(context.Background(), "host", "host_user", "")
	if err != nil {
		t.Fatal(err)
	}
	if err := room.Join(context.Background(), "other", "other_user", "", false); err != nil {
		t.Fatal(err)
	}
	now := time.Now().UTC()
	if err := room.Handle("host", rooms.Envelope{Type: "seat.select", Payload: json.RawMessage(`{"seat":"invalid"}`)}, now); err == nil {
		t.Fatal("invalid seat was accepted")
	}
	if err := room.Handle("host", rooms.Envelope{Type: "seat.select", Payload: json.RawMessage(`{"seat":"north"}`)}, now); err != nil {
		t.Fatal(err)
	}
	if err := room.Handle("other", rooms.Envelope{Type: "seat.select", Payload: json.RawMessage(`{"seat":"north"}`)}, now); err == nil {
		t.Fatal("occupied seat was accepted")
	}
	if room.Participants["other"].Seat.Valid() {
		t.Fatalf("rejected participant retained a seat: %s", room.Participants["other"].Seat)
	}
}

func TestRoomCommandsEnforceSpectatorAndHostPermissions(t *testing.T) {
	registry := rooms.NewRegistry(slog.Default(), nil)
	room, err := registry.Create(context.Background(), "host", "host_user", "")
	if err != nil {
		t.Fatal(err)
	}
	if err := room.Join(context.Background(), "spectator", "spectator_user", "", true); err != nil {
		t.Fatal(err)
	}
	if err := room.Handle("spectator", rooms.Envelope{Type: "seat.select", Payload: json.RawMessage(`{"seat":"north"}`)}, time.Now().UTC()); err == nil {
		t.Fatal("spectator was allowed to issue a command")
	}
	if err := room.Handle("spectator", rooms.Envelope{Type: "settings.update", Payload: json.RawMessage(`{"mode":"fully_visible","clock":"fast"}`)}, time.Now().UTC()); err == nil {
		t.Fatal("spectator was allowed to change settings")
	}
	if err := room.Handle("host", rooms.Envelope{Type: "seat.select", Payload: json.RawMessage(`{"seat":"north"}`)}, time.Now().UTC()); err != nil {
		t.Fatal(err)
	}
	if err := room.Handle("host", rooms.Envelope{Type: "settings.update", Payload: json.RawMessage(`{"mode":"fully_visible","clock":"fast"}`)}, time.Now().UTC()); err != nil {
		t.Fatal(err)
	}
	if room.State.Mode != game.FullyVisible || room.State.Clock != game.Fast {
		t.Fatalf("host settings were not applied: mode=%s clock=%s", room.State.Mode, room.State.Clock)
	}
}

func TestRoomRequestIDsMakeCommandsIdempotent(t *testing.T) {
	registry := rooms.NewRegistry(slog.Default(), nil)
	room, err := registry.Create(context.Background(), "host", "host_user", "")
	if err != nil {
		t.Fatal(err)
	}
	now := time.Now().UTC()
	request := rooms.Envelope{Type: "seat.select", RequestID: "same-request", Payload: json.RawMessage(`{"seat":"north"}`)}
	if err := room.Handle("host", request, now); err != nil {
		t.Fatal(err)
	}
	version := room.State.Version
	request.Payload = json.RawMessage(`{"seat":"east"}`)
	if err := room.Handle("host", request, now); err != nil {
		t.Fatal(err)
	}
	if room.Participants["host"].Seat != game.North || room.State.Version != version {
		t.Fatalf("duplicate request was applied twice: seat=%s version=%d want=%d", room.Participants["host"].Seat, room.State.Version, version)
	}
}

func TestRoomChangingSeatsRemovesTheOldDeployment(t *testing.T) {
	registry := rooms.NewRegistry(slog.Default(), nil)
	room, err := registry.Create(context.Background(), "host", "host_user", "")
	if err != nil {
		t.Fatal(err)
	}
	now := time.Now().UTC()
	if err := room.Handle("host", rooms.Envelope{Type: "seat.select", Payload: json.RawMessage(`{"seat":"north"}`)}, now); err != nil {
		t.Fatal(err)
	}
	if len(room.State.Pieces) != 25 {
		t.Fatalf("north default deployment size = %d", len(room.State.Pieces))
	}
	if err := room.Handle("host", rooms.Envelope{Type: "seat.select", Payload: json.RawMessage(`{"seat":"east"}`)}, now); err != nil {
		t.Fatal(err)
	}
	if room.Participants["host"].Seat != game.East || len(room.State.Pieces) != 25 {
		t.Fatalf("seat change did not replace deployment: seat=%s pieces=%d", room.Participants["host"].Seat, len(room.State.Pieces))
	}
	for node, piece := range room.State.Pieces {
		position, ok := room.Board.Node(node)
		if !ok || piece.Owner != game.East || position.DeployFor != game.East {
			t.Fatalf("old deployment remains at %s: position=%#v piece=%#v", node, position, piece)
		}
	}
}

func TestPlayerCanLeaveSeatAndRejoinAnotherSeat(t *testing.T) {
	registry := rooms.NewRegistry(slog.Default(), nil)
	room, err := registry.Create(context.Background(), "host", "host_user", "")
	if err != nil {
		t.Fatal(err)
	}
	now := time.Now().UTC()
	if err := room.Handle("host", rooms.Envelope{Type: "seat.select", Payload: json.RawMessage(`{"seat":"north"}`)}, now); err != nil {
		t.Fatal(err)
	}
	if len(room.State.Pieces) != 25 {
		t.Fatalf("seat selection did not create deployment: %d", len(room.State.Pieces))
	}
	if err := room.Handle("host", rooms.Envelope{Type: "seat.leave"}, now); err != nil {
		t.Fatal(err)
	}
	if room.Participants["host"].Seat.Valid() || room.State.Players[game.North].Username != "" || len(room.State.Pieces) != 0 {
		t.Fatalf("seat leave did not clear seat/deployment: participant=%#v player=%#v pieces=%d", room.Participants["host"], room.State.Players[game.North], len(room.State.Pieces))
	}
	if err := room.Handle("host", rooms.Envelope{Type: "seat.select", Payload: json.RawMessage(`{"seat":"east"}`)}, now); err != nil {
		t.Fatal(err)
	}
	if room.Participants["host"].Seat != game.East || len(room.State.Pieces) != 25 {
		t.Fatalf("player could not rejoin another seat: participant=%#v pieces=%d", room.Participants["host"], len(room.State.Pieces))
	}
}
