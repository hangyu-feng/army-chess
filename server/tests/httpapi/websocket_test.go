package httpapi_test

import (
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/cookiejar"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/coder/websocket"
	"github.com/fenghangyu/army-chess/server/internal/game"
	"github.com/fenghangyu/army-chess/server/internal/httpapi"
	"github.com/fenghangyu/army-chess/server/internal/persistence"
)

func TestWebSocketSnapshotAndSeatCommand(t *testing.T) {
	app := httpapi.New(slog.New(slog.NewTextHandler(io.Discard, nil)), (*persistence.DB)(nil), nil)
	server := httptest.NewServer(app.Routes())
	defer server.Close()
	jar, err := cookiejar.New(nil)
	if err != nil {
		t.Fatal(err)
	}
	client := server.Client()
	client.Jar = jar

	response, err := client.Post(server.URL+"/api/session", "application/json", strings.NewReader(`{"username":"ws_host"}`))
	if err != nil {
		t.Fatal(err)
	}
	response.Body.Close()
	if response.StatusCode != http.StatusCreated {
		t.Fatalf("session status: %d", response.StatusCode)
	}
	response, err = client.Post(server.URL+"/api/rooms", "application/json", strings.NewReader(`{}`))
	if err != nil {
		t.Fatal(err)
	}
	var room struct {
		Code string `json:"code"`
	}
	if err := json.NewDecoder(response.Body).Decode(&room); err != nil {
		t.Fatal(err)
	}
	response.Body.Close()
	if response.StatusCode != http.StatusCreated {
		t.Fatalf("room status: %d", response.StatusCode)
	}
	joinPath := server.URL + "/api/rooms/" + room.Code + "/join"
	response, err = client.Post(joinPath, "application/json", strings.NewReader(`{"spectator":false}`))
	if err != nil {
		t.Fatal(err)
	}
	response.Body.Close()
	if response.StatusCode != http.StatusOK {
		t.Fatalf("join status: %d", response.StatusCode)
	}

	cookies := jar.Cookies(mustURL(t, server.URL))
	header := http.Header{}
	for _, cookie := range cookies {
		header.Add("Cookie", cookie.String())
	}
	wsURL := "ws" + strings.TrimPrefix(server.URL, "http") + "/api/rooms/" + room.Code + "/ws"
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	conn, _, err := websocket.Dial(ctx, wsURL, &websocket.DialOptions{HTTPHeader: header})
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close(websocket.StatusNormalClosure, "test complete")

	var first struct {
		Type    string    `json:"type"`
		Payload game.View `json:"payload"`
	}
	if _, data, err := conn.Read(ctx); err != nil {
		t.Fatal(err)
	} else if err := json.Unmarshal(data, &first); err != nil {
		t.Fatal(err)
	} else if first.Type != "snapshot" {
		t.Fatalf("first realtime message: %s", first.Type)
	}

	command, _ := json.Marshal(map[string]any{
		"type": "seat.select", "requestId": "seat-1", "payload": map[string]string{"seat": "north"},
	})
	if err := conn.Write(ctx, websocket.MessageText, command); err != nil {
		t.Fatal(err)
	}
	var second struct {
		Type    string    `json:"type"`
		Payload game.View `json:"payload"`
	}
	if _, data, err := conn.Read(ctx); err != nil {
		t.Fatal(err)
	} else if err := json.Unmarshal(data, &second); err != nil {
		t.Fatal(err)
	}
	if second.Type != "snapshot" || len(second.Payload.Pieces) != 25 {
		t.Fatalf("seat snapshot: type=%s pieces=%d", second.Type, len(second.Payload.Pieces))
	}
	if second.Payload.Pieces["north-r6-2L"].Kind != game.Flag {
		t.Fatalf("owner projection did not include the flag rank")
	}
}

func mustURL(t *testing.T, raw string) *url.URL {
	t.Helper()
	parsed, err := url.Parse(raw)
	if err != nil {
		t.Fatal(err)
	}
	return parsed
}
