package httpapi_test

import (
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/fenghangyu/army-chess/server/internal/httpapi"
	"github.com/fenghangyu/army-chess/server/internal/persistence"
)

func TestSessionAndRoomHTTPFlow(t *testing.T) {
	app := httpapi.New(slog.New(slog.NewTextHandler(io.Discard, nil)), (*persistence.DB)(nil), nil)
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

	request = httptest.NewRequest(http.MethodPost, "/api/rooms/"+room.Code+"/join", strings.NewReader(`{"spectator":false}`))
	request.Header.Set("Content-Type", "application/json")
	request.AddCookie(cookie)
	response = httptest.NewRecorder()
	routes.ServeHTTP(response, request)
	if response.Code != http.StatusOK {
		t.Fatalf("room join status: %d", response.Code)
	}
	var joined struct {
		Participants []struct {
			Username string `json:"username"`
			Seat     string `json:"seat"`
			Role     string `json:"role"`
		} `json:"participants"`
	}
	if err := json.NewDecoder(response.Body).Decode(&joined); err != nil {
		t.Fatal(err)
	}
	if len(joined.Participants) != 1 || joined.Participants[0].Username != "red_cedar" || joined.Participants[0].Role != "spectator" || joined.Participants[0].Seat != "" {
		t.Fatalf("join did not return spectator-first roster: %#v", joined.Participants)
	}
}

func TestInvalidUsernameIsRejected(t *testing.T) {
	for _, username := range []string{"Not Valid", "ab", "_abc", "abc_", "abc-def", "abc.def", "abcdefghijklmnopqrs_tuv"} {
		t.Run(username, func(t *testing.T) {
			app := httpapi.New(slog.Default(), (*persistence.DB)(nil), nil)
			request := httptest.NewRequest(http.MethodPost, "/api/session", strings.NewReader(`{"username":"`+username+`"}`))
			request.Header.Set("Content-Type", "application/json")
			response := httptest.NewRecorder()
			app.Routes().ServeHTTP(response, request)
			if response.Code != http.StatusBadRequest {
				t.Fatalf("invalid username status: %d", response.Code)
			}
		})
	}
}

func TestSessionNormalizesValidUsernameAndCanBeDeleted(t *testing.T) {
	app := httpapi.New(slog.Default(), (*persistence.DB)(nil), nil)
	request := httptest.NewRequest(http.MethodPost, "/api/session", strings.NewReader(`{"username":" BAI_HUA "}`))
	request.Header.Set("Content-Type", "application/json")
	response := httptest.NewRecorder()
	app.Routes().ServeHTTP(response, request)
	if response.Code != http.StatusCreated {
		t.Fatalf("normalized session status: %d", response.Code)
	}
	var session struct {
		Username string `json:"username"`
	}
	if err := json.NewDecoder(response.Body).Decode(&session); err != nil {
		t.Fatal(err)
	}
	if session.Username != "bai_hua" {
		t.Fatalf("normalized username = %q", session.Username)
	}
	cookies := response.Result().Cookies()
	if len(cookies) != 1 || cookies[0].Name != "army_session" || cookies[0].Value == "" {
		t.Fatalf("session cookie = %#v", cookies)
	}

	request = httptest.NewRequest(http.MethodDelete, "/api/session", nil)
	request.AddCookie(cookies[0])
	response = httptest.NewRecorder()
	app.Routes().ServeHTTP(response, request)
	if response.Code != http.StatusNoContent {
		t.Fatalf("delete session status: %d", response.Code)
	}
	deleted := response.Result().Cookies()
	if len(deleted) != 1 || deleted[0].MaxAge >= 0 || deleted[0].Value != "" {
		t.Fatalf("session deletion cookie = %#v", deleted)
	}
}

func TestHTTPRejectsMalformedJSONAndUnauthenticatedRoomAccess(t *testing.T) {
	app := httpapi.New(slog.Default(), (*persistence.DB)(nil), nil)
	routes := app.Routes()

	request := httptest.NewRequest(http.MethodPost, "/api/session", strings.NewReader("{"))
	request.Header.Set("Content-Type", "application/json")
	response := httptest.NewRecorder()
	routes.ServeHTTP(response, request)
	if response.Code != http.StatusBadRequest {
		t.Fatalf("malformed session status: %d", response.Code)
	}

	for _, path := range []string{"/api/rooms", "/api/rooms/ABCDEFGH/join", "/api/rooms/ABCDEFGH/ws"} {
		request = httptest.NewRequest(http.MethodPost, path, strings.NewReader(`{}`))
		if strings.HasSuffix(path, "/ws") {
			request = httptest.NewRequest(http.MethodGet, path, nil)
		}
		response = httptest.NewRecorder()
		routes.ServeHTTP(response, request)
		if response.Code != http.StatusUnauthorized {
			t.Fatalf("%s status = %d, want 401", path, response.Code)
		}
	}

	request = httptest.NewRequest(http.MethodGet, "/api/rooms/MISSING1", nil)
	response = httptest.NewRecorder()
	routes.ServeHTTP(response, request)
	if response.Code != http.StatusNotFound {
		t.Fatalf("missing room status: %d", response.Code)
	}
}

func TestClientRoutesServeTheAppAndOnlyEightCharacterPathsAreRooms(t *testing.T) {
	static := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/" {
			t.Fatalf("static app path = %q, want /", r.URL.Path)
		}
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("index"))
	})
	app := httpapi.New(slog.Default(), (*persistence.DB)(nil), static)
	for _, path := range []string{"/ABC1DEFG", "/replay/match-1", "/profile/baihua"} {
		request := httptest.NewRequest(http.MethodGet, path, nil)
		response := httptest.NewRecorder()
		app.Routes().ServeHTTP(response, request)
		if response.Code != http.StatusOK || response.Body.String() != "index" {
			t.Fatalf("%s response = %d %q", path, response.Code, response.Body.String())
		}
	}
	for _, path := range []string{"/ABC1234", "/ABCDEFGH", "/2ABC3456"} {
		request := httptest.NewRequest(http.MethodGet, path, nil)
		response := httptest.NewRecorder()
		app.Routes().ServeHTTP(response, request)
		if response.Code != http.StatusNotFound {
			t.Fatalf("invalid room path %s status = %d, want 404", path, response.Code)
		}
	}
}

func TestConfiguredPublicBaseURL(t *testing.T) {
	t.Setenv("PUBLIC_BASE_URL", "https://chess.example.com/")
	app := httpapi.New(slog.Default(), (*persistence.DB)(nil), nil)
	request := httptest.NewRequest(http.MethodGet, "/api/config", nil)
	response := httptest.NewRecorder()
	app.Routes().ServeHTTP(response, request)
	var config struct {
		BaseURL string `json:"baseUrl"`
	}
	if err := json.NewDecoder(response.Body).Decode(&config); err != nil {
		t.Fatal(err)
	}
	if response.Code != http.StatusOK || config.BaseURL != "https://chess.example.com" {
		t.Fatalf("config response: %d %#v", response.Code, config)
	}
}

func TestConfigBuildsBaseURLFromForwardedRequestWhenUnset(t *testing.T) {
	app := httpapi.New(slog.Default(), (*persistence.DB)(nil), nil)
	request := httptest.NewRequest(http.MethodGet, "/api/config", nil)
	request.Host = "play.example.test:8443"
	request.Header.Set("X-Forwarded-Proto", "https")
	response := httptest.NewRecorder()
	app.Routes().ServeHTTP(response, request)
	var config struct {
		BaseURL string `json:"baseUrl"`
	}
	if err := json.NewDecoder(response.Body).Decode(&config); err != nil {
		t.Fatal(err)
	}
	if config.BaseURL != "https://play.example.test:8443" {
		t.Fatalf("forwarded base URL = %q", config.BaseURL)
	}
}

func TestHTTPAddsRequestIDAndSecurityHeaders(t *testing.T) {
	app := httpapi.New(slog.Default(), (*persistence.DB)(nil), nil)
	request := httptest.NewRequest(http.MethodGet, "/healthz", nil)
	request.Header.Set("X-Request-ID", "request-test-123")
	response := httptest.NewRecorder()
	app.Routes().ServeHTTP(response, request)
	if response.Code != http.StatusOK || response.Header().Get("X-Request-ID") != "request-test-123" {
		t.Fatalf("health/request id response: status=%d id=%q", response.Code, response.Header().Get("X-Request-ID"))
	}
	for header, want := range map[string]string{
		"X-Content-Type-Options": "nosniff",
		"X-Frame-Options":        "DENY",
		"Referrer-Policy":        "same-origin",
	} {
		if got := response.Header().Get(header); got != want {
			t.Fatalf("%s = %q, want %q", header, got, want)
		}
	}
}
