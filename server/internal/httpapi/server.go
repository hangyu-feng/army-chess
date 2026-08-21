package httpapi

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"log/slog"
	"net/http"
	"os"
	"regexp"
	"strings"
	"sync"
	"time"

	"github.com/coder/websocket"
	"github.com/fenghangyu/army-chess/server/internal/persistence"
	"github.com/fenghangyu/army-chess/server/internal/rooms"
	"github.com/go-chi/chi/v5"
)

var usernamePattern = regexp.MustCompile(`^[a-z][a-z_]{1,18}[a-z]$`)

type Session struct {
	ID        string
	ProfileID string
	Username  string
	CreatedAt time.Time
	LastSeen  time.Time
}

type Server struct {
	Logger   *slog.Logger
	Rooms    *rooms.Registry
	DB       *persistence.DB
	Sessions map[string]*Session
	Mu       sync.RWMutex
	Static   http.Handler
}

func New(logger *slog.Logger, db *persistence.DB, static http.Handler) *Server {
	return &Server{Logger: logger, DB: db, Rooms: rooms.NewRegistry(logger, db), Sessions: map[string]*Session{}, Static: static}
}

func (s *Server) Routes() http.Handler {
	r := chi.NewRouter()
	r.Use(requestID)
	r.Use(s.corsAndHeaders)
	r.Get("/healthz", s.healthz)
	r.Get("/readyz", s.readyz)
	r.Route("/api", func(r chi.Router) {
		r.Post("/session", s.createSession)
		r.Delete("/session", s.deleteSession)
		r.Get("/me", s.me)
		r.Post("/rooms", s.requireSession(s.createRoom))
		r.Get("/rooms/{code}", s.getRoom)
		r.Post("/rooms/{code}/join", s.requireSession(s.joinRoom))
		r.Get("/rooms/{code}/ws", s.websocket)
		r.Get("/matches/{id}/replay", s.replay)
		r.Get("/profiles/{username}", s.profile)
		r.Get("/profiles/{username}/matches", s.profileMatches)
		r.Get("/layouts", s.requireSession(s.listLayouts))
		r.Post("/layouts", s.requireSession(s.createLayout))
		r.Put("/layouts/{id}", s.requireSession(s.updateLayout))
		r.Delete("/layouts/{id}", s.requireSession(s.deleteLayout))
	})
	if s.Static != nil {
		r.Handle("/*", s.Static)
	}
	return r
}

func (s *Server) createSession(w http.ResponseWriter, r *http.Request) {
	var payload struct {
		Username string `json:"username"`
	}
	if !decodeJSON(w, r, &payload) {
		return
	}
	payload.Username = strings.ToLower(strings.TrimSpace(payload.Username))
	if !usernamePattern.MatchString(payload.Username) {
		writeError(w, http.StatusBadRequest, "username must match ^[a-z][a-z_]{1,18}[a-z]$")
		return
	}
	id := newToken()
	now := time.Now().UTC()
	identity, err := s.DB.CreateSession(r.Context(), id, payload.Username)
	if err != nil {
		writeError(w, http.StatusServiceUnavailable, "could not persist session")
		return
	}
	s.Mu.Lock()
	s.Sessions[id] = &Session{ID: id, ProfileID: identity.ProfileID, Username: payload.Username, CreatedAt: now, LastSeen: now}
	s.Mu.Unlock()
	http.SetCookie(w, &http.Cookie{Name: "army_session", Value: id, Path: "/", HttpOnly: true, Secure: os.Getenv("COOKIE_SECURE") != "false", SameSite: http.SameSiteLaxMode, MaxAge: 60 * 60 * 24 * 30})
	writeJSON(w, http.StatusCreated, map[string]any{"username": payload.Username})
}

func (s *Server) deleteSession(w http.ResponseWriter, r *http.Request) {
	if session := s.session(r); session != nil {
		s.Mu.Lock()
		delete(s.Sessions, session.ID)
		s.Mu.Unlock()
		_ = s.DB.DeleteSession(r.Context(), session.ID)
	}
	http.SetCookie(w, &http.Cookie{Name: "army_session", Value: "", Path: "/", MaxAge: -1, HttpOnly: true, Secure: os.Getenv("COOKIE_SECURE") != "false", SameSite: http.SameSiteLaxMode})
	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) me(w http.ResponseWriter, r *http.Request) {
	session := s.session(r)
	if session == nil {
		writeError(w, http.StatusUnauthorized, "not signed in")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"username": session.Username})
}

func (s *Server) createRoom(w http.ResponseWriter, r *http.Request) {
	session := mustSession(r)
	room, err := s.Rooms.Create(r.Context(), session.ID, session.Username, session.ProfileID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusCreated, room.PublicInfo())
}

func (s *Server) getRoom(w http.ResponseWriter, r *http.Request) {
	room, ok := s.Rooms.Get(chi.URLParam(r, "code"))
	if !ok {
		writeError(w, http.StatusNotFound, "room not found")
		return
	}
	writeJSON(w, http.StatusOK, room.PublicInfo())
}

func (s *Server) joinRoom(w http.ResponseWriter, r *http.Request) {
	session := mustSession(r)
	room, ok := s.Rooms.Get(chi.URLParam(r, "code"))
	if !ok {
		writeError(w, http.StatusNotFound, "room not found")
		return
	}
	var payload struct {
		Spectator bool `json:"spectator"`
	}
	if !decodeJSON(w, r, &payload) {
		return
	}
	if err := room.Join(r.Context(), session.ID, session.Username, session.ProfileID, payload.Spectator); err != nil {
		writeError(w, http.StatusConflict, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, room.PublicInfo())
}

func (s *Server) websocket(w http.ResponseWriter, r *http.Request) {
	session := s.session(r)
	if session == nil {
		writeError(w, http.StatusUnauthorized, "not signed in")
		return
	}
	room, ok := s.Rooms.Get(chi.URLParam(r, "code"))
	if !ok {
		writeError(w, http.StatusNotFound, "room not found")
		return
	}
	conn, err := websocket.Accept(w, r, &websocket.AcceptOptions{})
	if err != nil {
		return
	}
	defer conn.Close(websocket.StatusNormalClosure, "closed")
	room.RunSocket(r.Context(), session.ID, session.Username, session.ProfileID, conn)
}

func (s *Server) replay(w http.ResponseWriter, r *http.Request) {
	events, err := s.DB.LoadReplay(r.Context(), chi.URLParam(r, "id"))
	if err != nil {
		writeError(w, http.StatusNotFound, "replay not found")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"matchId": chi.URLParam(r, "id"), "events": events})
}

func (s *Server) profile(w http.ResponseWriter, r *http.Request) {
	summary, err := s.DB.GetProfileSummary(r.Context(), chi.URLParam(r, "username"))
	if err != nil {
		writeError(w, http.StatusNotFound, "profile not found")
		return
	}
	writeJSON(w, http.StatusOK, summary)
}

func (s *Server) profileMatches(w http.ResponseWriter, r *http.Request) {
	matches, err := s.DB.ListProfileMatches(r.Context(), chi.URLParam(r, "username"))
	if err != nil {
		writeError(w, http.StatusNotFound, "profile not found")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"matches": matches})
}

type layoutPayload struct {
	Name       string          `json:"name"`
	Deployment json.RawMessage `json:"deployment"`
}

func (s *Server) listLayouts(w http.ResponseWriter, r *http.Request) {
	layouts, err := s.DB.ListSavedLayouts(r.Context(), mustSession(r).ProfileID)
	if err != nil {
		writeError(w, http.StatusServiceUnavailable, "could not load layouts")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"layouts": layouts})
}

func (s *Server) createLayout(w http.ResponseWriter, r *http.Request) {
	var payload layoutPayload
	if !decodeJSON(w, r, &payload) {
		return
	}
	if strings.TrimSpace(payload.Name) == "" || len(payload.Deployment) == 0 || string(payload.Deployment) == "null" {
		writeError(w, http.StatusBadRequest, "name and deployment are required")
		return
	}
	layout, err := s.DB.CreateSavedLayout(r.Context(), mustSession(r).ProfileID, strings.TrimSpace(payload.Name), payload.Deployment)
	if err != nil {
		writeError(w, http.StatusConflict, "could not save layout")
		return
	}
	writeJSON(w, http.StatusCreated, layout)
}

func (s *Server) updateLayout(w http.ResponseWriter, r *http.Request) {
	var payload layoutPayload
	if !decodeJSON(w, r, &payload) {
		return
	}
	layout, err := s.DB.UpdateSavedLayout(r.Context(), mustSession(r).ProfileID, chi.URLParam(r, "id"), strings.TrimSpace(payload.Name), payload.Deployment)
	if err != nil {
		writeError(w, http.StatusNotFound, "layout not found")
		return
	}
	writeJSON(w, http.StatusOK, layout)
}

func (s *Server) deleteLayout(w http.ResponseWriter, r *http.Request) {
	if err := s.DB.DeleteSavedLayout(r.Context(), mustSession(r).ProfileID, chi.URLParam(r, "id")); err != nil {
		writeError(w, http.StatusNotFound, "layout not found")
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) requireSession(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if s.session(r) == nil {
			writeError(w, http.StatusUnauthorized, "not signed in")
			return
		}
		next(w, r.WithContext(context.WithValue(r.Context(), sessionKey{}, s.session(r))))
	}
}

func (s *Server) session(r *http.Request) *Session {
	cookie, err := r.Cookie("army_session")
	if err != nil || cookie.Value == "" {
		return nil
	}
	s.Mu.RLock()
	session := s.Sessions[cookie.Value]
	s.Mu.RUnlock()
	if session == nil {
		ctx, cancel := context.WithTimeout(r.Context(), 2*time.Second)
		identity, err := s.DB.LookupSession(ctx, cookie.Value)
		cancel()
		if err == nil {
			session = &Session{ID: cookie.Value, ProfileID: identity.ProfileID, Username: identity.Username, LastSeen: time.Now().UTC()}
			s.Mu.Lock()
			s.Sessions[cookie.Value] = session
			s.Mu.Unlock()
		}
	}
	if session != nil {
		s.Mu.Lock()
		session.LastSeen = time.Now().UTC()
		s.Mu.Unlock()
	}
	return session
}

func mustSession(r *http.Request) *Session {
	value := r.Context().Value(sessionKey{})
	return value.(*Session)
}

func (s *Server) healthz(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

func (s *Server) readyz(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), 2*time.Second)
	defer cancel()
	if err := s.DB.Ping(ctx); err != nil {
		writeError(w, http.StatusServiceUnavailable, "database unavailable")
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "ready"})
}

func (s *Server) corsAndHeaders(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-Content-Type-Options", "nosniff")
		w.Header().Set("X-Frame-Options", "DENY")
		w.Header().Set("Referrer-Policy", "same-origin")
		w.Header().Set("Content-Security-Policy", "default-src 'self'; connect-src 'self' ws: wss:; img-src 'self' data:; style-src 'self'; script-src 'self'")
		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusNoContent)
			return
		}
		next.ServeHTTP(w, r)
	})
}

type sessionKey struct{}

func requestID(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		id := r.Header.Get("X-Request-ID")
		if id == "" {
			id = newToken()[:12]
		}
		w.Header().Set("X-Request-ID", id)
		next.ServeHTTP(w, r.WithContext(context.WithValue(r.Context(), requestIDKey{}, id)))
	})
}

type requestIDKey struct{}

func decodeJSON(w http.ResponseWriter, r *http.Request, target any) bool {
	defer r.Body.Close()
	decoder := json.NewDecoder(http.MaxBytesReader(w, r.Body, 1<<20))
	if err := decoder.Decode(target); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON payload")
		return false
	}
	return true
}

func writeJSON(w http.ResponseWriter, status int, value any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(value)
}

func writeError(w http.ResponseWriter, status int, message string) {
	writeJSON(w, status, map[string]any{"error": message})
}

func newToken() string {
	var raw [24]byte
	if _, err := rand.Read(raw[:]); err != nil {
		return time.Now().UTC().Format("20060102150405.000000000")
	}
	return hex.EncodeToString(raw[:])
}
