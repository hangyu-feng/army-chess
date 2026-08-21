package httpapi

import (
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/fenghangyu/army-chess/server/internal/persistence"
)

func TestSessionAndRoomHTTPFlow(t *testing.T) {
	app := New(slog.New(slog.NewTextHandler(io.Discard, nil)), (*persistence.DB)(nil), nil)
	routes := app.Routes()
	request := httptest.NewRequest(http.MethodPost, "/api/session", strings.NewReader(`{"username":"red_cedar"}`))
	request.Header.Set("Content-Type", "application/json")
	response := httptest.NewRecorder()
	routes.ServeHTTP(response, request)
	if response.Code != http.StatusCreated {
		t.Fatalf("session status: %d", response.Code)
	}
	cookie := response.Result().Cookies()[0]

	request = httptest.NewRequest(http.MethodPost, "/api/rooms", strings.NewReader(`{}`))
	request.AddCookie(cookie)
	response = httptest.NewRecorder()
	routes.ServeHTTP(response, request)
	var room struct {
		Code string `json:"code"`
	}
	if err := json.NewDecoder(response.Body).Decode(&room); err != nil {
		t.Fatal(err)
	}
	if response.Code != http.StatusCreated || len(room.Code) != 8 {
		t.Fatalf("room response: %d %#v", response.Code, room)
	}

	request = httptest.NewRequest(http.MethodGet, "/api/rooms/"+room.Code, nil)
	response = httptest.NewRecorder()
	routes.ServeHTTP(response, request)
	if response.Code != http.StatusOK {
		t.Fatalf("room lookup status: %d", response.Code)
	}
}

func TestInvalidUsernameIsRejected(t *testing.T) {
	app := New(slog.Default(), (*persistence.DB)(nil), nil)
	request := httptest.NewRequest(http.MethodPost, "/api/session", strings.NewReader(`{"username":"Not Valid"}`))
	request.Header.Set("Content-Type", "application/json")
	response := httptest.NewRecorder()
	app.Routes().ServeHTTP(response, request)
	if response.Code != http.StatusBadRequest {
		t.Fatalf("invalid username status: %d", response.Code)
	}
}
