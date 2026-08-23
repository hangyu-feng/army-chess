package rooms

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/coder/websocket"
	"github.com/fenghangyu/army-chess/server/internal/game"
	"github.com/fenghangyu/army-chess/server/internal/persistence"
)

const codeLetters = "ABCDEFGHJKLMNPQRSTUVWXYZ"
const codeDigits = "23456789"
const codeAlphabet = codeLetters + codeDigits

type Participant struct {
	ID        string
	Username  string
	Seat      game.Seat
	Spectator bool
	Ready     bool
	Connected bool
	Conn      *websocket.Conn
	JoinedAt  time.Time
	ProfileID string
	writeMu   sync.Mutex
}

type participantState struct {
	ID, Username, ProfileID string
	Seat                    game.Seat
	Spectator, Ready        bool
	Connected               bool
	Conn                    *websocket.Conn
	JoinedAt                time.Time
}

func snapshotParticipant(p *Participant) participantState {
	return participantState{ID: p.ID, Username: p.Username, ProfileID: p.ProfileID, Seat: p.Seat, Spectator: p.Spectator, Ready: p.Ready, Connected: p.Connected, Conn: p.Conn, JoinedAt: p.JoinedAt}
}

func restoreParticipant(p *Participant, state participantState) {
	p.ID, p.Username, p.ProfileID, p.Seat = state.ID, state.Username, state.ProfileID, state.Seat
	p.Spectator, p.Ready, p.Connected, p.Conn, p.JoinedAt = state.Spectator, state.Ready, state.Connected, state.Conn, state.JoinedAt
}

type Room struct {
	mu           sync.Mutex
	Code         string
	HostID       string
	State        *game.State
	Board        *game.Board
	Participants map[string]*Participant
	Logger       *slog.Logger
	DB           *persistence.DB
	PersistentID string
	MatchID      string
	Sequence     int64
	SeenRequests map[string]map[string]bool
	RematchReady map[game.Seat]bool
}

type Registry struct {
	mu     sync.RWMutex
	rooms  map[string]*Room
	logger *slog.Logger
	db     *persistence.DB
}

func NewRegistry(logger *slog.Logger, db *persistence.DB) *Registry {
	return &Registry{rooms: map[string]*Room{}, logger: logger, db: db}
}

func (r *Registry) Create(ctx context.Context, hostID, username, profileID string) (*Room, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	for attempt := 0; attempt < 10; attempt++ {
		code, err := newRoomCode()
		if err != nil {
			return nil, err
		}
		if _, exists := r.rooms[code]; exists {
			continue
		}
		now := time.Now().UTC()
		room := &Room{
			Code: code, HostID: hostID, Board: game.NewBoard(),
			State:        game.NewState(game.FourDark, game.Standard, randomOpening(), now),
			Participants: map[string]*Participant{}, Logger: r.logger, DB: r.db, SeenRequests: map[string]map[string]bool{}, RematchReady: map[game.Seat]bool{},
		}
		if r.db != nil {
			persistentID, err := r.db.CreateRoom(ctx, code, profileID, string(room.State.Mode), string(room.State.Clock))
			if err != nil {
				return nil, err
			}
			room.PersistentID = persistentID
			matchID, err := r.db.CreateMatch(ctx, persistentID, string(room.State.Mode), string(room.State.Clock), string(room.State.Opening))
			if err != nil {
				return nil, err
			}
			room.MatchID = matchID
		}
		room.Participants[hostID] = &Participant{ID: hostID, Username: username, ProfileID: profileID, Spectator: true, Connected: true, JoinedAt: now}
		if err := room.persistParticipantLocked(ctx, room.Participants[hostID]); err != nil {
			return nil, err
		}
		r.rooms[code] = room
		return room, nil
	}
	return nil, errors.New("could not allocate a room code")
}

func (r *Registry) Get(code string) (*Room, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	room, ok := r.rooms[strings.ToUpper(code)]
	return room, ok
}

func (r *Registry) Recover(ctx context.Context) error {
	if r.db == nil {
		return nil
	}
	persisted, err := r.db.LoadActiveRooms(ctx)
	if err != nil {
		return err
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	for _, saved := range persisted {
		if _, exists := r.rooms[saved.InviteCode]; exists {
			continue
		}
		state := game.NewState(game.VisibilityMode(saved.Mode), game.ClockPreset(saved.Clock), game.Seat(saved.Opening), time.Now().UTC())
		if string(saved.State) != "" && string(saved.State) != "{}" {
			if err := json.Unmarshal(saved.State, state); err != nil {
				return fmt.Errorf("decode room %s state: %w", saved.InviteCode, err)
			}
		}
		if state.Players == nil {
			state.Players = map[game.Seat]game.Player{}
		}
		if state.Pieces == nil {
			state.Pieces = map[string]game.Piece{}
		}
		if state.RevealedFlags == nil {
			state.RevealedFlags = map[game.Seat]string{}
		}
		if state.DrawAccepts == nil {
			state.DrawAccepts = map[game.Seat]bool{}
		}
		for _, seat := range game.Seats {
			if _, ok := state.Players[seat]; !ok {
				state.Players[seat] = game.Player{}
			}
		}
		room := &Room{
			Code: saved.InviteCode, HostID: "profile:" + saved.HostProfileID,
			State: state, Board: game.NewBoard(), Participants: map[string]*Participant{},
			Logger: r.logger, DB: r.db, PersistentID: saved.ID, MatchID: saved.MatchID, Sequence: saved.Sequence,
			SeenRequests: map[string]map[string]bool{},
			RematchReady: map[game.Seat]bool{},
		}
		participants, err := r.db.LoadRoomParticipants(ctx, saved.ID)
		if err != nil {
			return err
		}
		for _, savedParticipant := range participants {
			id := "profile:" + savedParticipant.ProfileID
			seat := game.Seat(savedParticipant.Seat)
			participant := &Participant{ID: id, ProfileID: savedParticipant.ProfileID, Username: savedParticipant.Username, Seat: seat, Spectator: savedParticipant.Role != "player" || !seat.Valid(), Ready: false, Connected: false, JoinedAt: time.Now().UTC()}
			room.Participants[id] = participant
			if seat.Valid() {
				player := state.Players[seat]
				player.Username = savedParticipant.Username
				player.Ready = savedParticipant.Ready
				player.Connected = false
				state.Players[seat] = player
			}
		}
		r.rooms[saved.InviteCode] = room
	}
	return nil
}

func (r *Registry) Tick(ctx context.Context) {
	r.mu.RLock()
	rooms := make([]*Room, 0, len(r.rooms))
	for _, room := range r.rooms {
		rooms = append(rooms, room)
	}
	r.mu.RUnlock()
	for _, room := range rooms {
		if room.Tick(ctx, time.Now().UTC()) {
			room.Broadcast(ctx)
		}
	}
}

func newRoomCode() (string, error) {
	bytes := make([]byte, 8)
	if _, err := rand.Read(bytes); err != nil {
		return "", err
	}
	bytes[0] = codeLetters[int(bytes[0])%len(codeLetters)]
	hasDigit := false
	for i := 1; i < len(bytes); i++ {
		bytes[i] = codeAlphabet[int(bytes[i])%len(codeAlphabet)]
		if strings.ContainsRune(codeDigits, rune(bytes[i])) {
			hasDigit = true
		}
	}
	if !hasDigit {
		bytes[1] = codeDigits[int(bytes[1])%len(codeDigits)]
	}
	return string(bytes), nil
}

func randomOpening() game.Seat {
	var b [1]byte
	if _, err := rand.Read(b[:]); err != nil {
		return game.North
	}
	return game.Seats[int(b[0])%len(game.Seats)]
}

func (r *Room) PublicInfo() map[string]any {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.publicInfoLocked()
}

func (r *Room) publicInfoLocked() map[string]any {
	participants := r.participantViewsLocked("")
	players := make([]map[string]any, 0, len(participants))
	for _, participant := range participants {
		players = append(players, map[string]any{
			"username": participant.Username, "seat": participant.Seat,
			"role": participant.Role, "connected": participant.Connected,
		})
	}
	hostUsername := ""
	if host, ok := r.Participants[r.HostID]; ok {
		hostUsername = host.Username
	}
	return map[string]any{
		"code": r.Code, "hostUsername": hostUsername, "phase": r.State.Phase,
		"mode": r.State.Mode, "clock": r.State.Clock, "opening": r.State.Opening,
		"participants": players, "spectatorCap": 50,
	}
}

func (r *Room) participantViewsLocked(selfID string) []game.ParticipantView {
	participants := make([]game.ParticipantView, 0, len(r.Participants))
	for _, participant := range r.Participants {
		role := "spectator"
		if !participant.Spectator && participant.Seat.Valid() {
			role = "player"
		}
		participants = append(participants, game.ParticipantView{
			Username:  participant.Username,
			Seat:      participant.Seat,
			Role:      role,
			Connected: participant.Connected,
			Self:      participant.ID == selfID,
		})
	}
	sort.SliceStable(participants, func(i, j int) bool {
		left, right := participants[i], participants[j]
		if left.Seat.Valid() != right.Seat.Valid() {
			return left.Seat.Valid()
		}
		if left.Seat.Valid() && right.Seat.Valid() {
			return seatOrder(left.Seat) < seatOrder(right.Seat)
		}
		return left.Username < right.Username
	})
	return participants
}

func seatOrder(seat game.Seat) int {
	for index, candidate := range game.Seats {
		if candidate == seat {
			return index
		}
	}
	return len(game.Seats)
}

func (r *Room) Join(ctx context.Context, id, username, profileID string, _ bool) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if existing, ok := r.Participants[id]; ok {
		old := snapshotParticipant(existing)
		existing.Username = username
		existing.ProfileID = profileID
		if !existing.Seat.Valid() {
			existing.Spectator = true
		}
		existing.Connected = true
		if err := r.persistParticipantLocked(ctx, existing); err != nil {
			restoreParticipant(existing, old)
			return err
		}
		return nil
	}
	for oldID, existing := range r.Participants {
		if profileID != "" && existing.ProfileID == profileID {
			old := snapshotParticipant(existing)
			delete(r.Participants, oldID)
			existing.ID, existing.Username, existing.Connected = id, username, true
			if !existing.Seat.Valid() {
				existing.Spectator = true
			}
			r.Participants[id] = existing
			if r.HostID == oldID {
				r.HostID = id
			}
			if err := r.persistParticipantLocked(ctx, existing); err != nil {
				delete(r.Participants, id)
				r.Participants[oldID] = existing
				restoreParticipant(existing, old)
				return err
			}
			return nil
		}
	}
	count := 0
	for _, p := range r.Participants {
		if p.Spectator {
			count++
		}
	}
	if count >= 50 {
		return errors.New("spectator limit reached")
	}
	// Every new room member starts in the spectator list. Taking a seat is an
	// explicit realtime action and changes the participant's role to player.
	r.Participants[id] = &Participant{ID: id, Username: username, ProfileID: profileID, Spectator: true, Connected: true, JoinedAt: time.Now().UTC()}
	if err := r.persistParticipantLocked(ctx, r.Participants[id]); err != nil {
		delete(r.Participants, id)
		return err
	}
	return nil
}

func (r *Room) Attach(id string, conn *websocket.Conn) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	participant, ok := r.Participants[id]
	if !ok {
		return errors.New("join the room before opening realtime connection")
	}
	participant.Conn = conn
	participant.Connected = true
	if participant.Seat.Valid() {
		player := r.State.Players[participant.Seat]
		player.Connected = true
		r.State.Players[participant.Seat] = player
	}
	return nil
}

func (r *Room) AttachOrRestore(id, username, profileID string, conn *websocket.Conn) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	participant, ok := r.Participants[id]
	if !ok && profileID != "" {
		for oldID, candidate := range r.Participants {
			if candidate.ProfileID == profileID {
				delete(r.Participants, oldID)
				candidate.ID = id
				candidate.Username = username
				r.Participants[id] = candidate
				if r.HostID == oldID {
					r.HostID = id
				}
				participant, ok = candidate, true
				break
			}
		}
	}
	if !ok {
		return errors.New("join the room before opening realtime connection")
	}
	participant.Conn = conn
	participant.Connected = true
	if participant.Seat.Valid() {
		player := r.State.Players[participant.Seat]
		player.Connected = true
		r.State.Players[participant.Seat] = player
	}
	return nil
}

func (r *Room) Detach(id string, conn *websocket.Conn) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if participant, ok := r.Participants[id]; ok {
		if participant.Conn != conn {
			return
		}
		participant.Conn = nil
		participant.Connected = false
		if participant.Seat.Valid() {
			player := r.State.Players[participant.Seat]
			player.Connected = false
			r.State.Players[participant.Seat] = player
		}
		if r.HostID == id {
			var successor *Participant
			for _, candidate := range r.Participants {
				if candidate.ID == id || !candidate.Connected || candidate.Spectator {
					continue
				}
				if successor == nil || candidate.JoinedAt.Before(successor.JoinedAt) {
					successor = candidate
				}
			}
			if successor != nil {
				r.HostID = successor.ID
			}
		}
	}
}

func (r *Room) ViewFor(id string) (game.View, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	participant, ok := r.Participants[id]
	if !ok {
		return game.View{}, errors.New("participant not found")
	}
	view := r.State.Project(game.Viewer{Seat: participant.Seat, Spectator: participant.Spectator}, r.Board)
	view.Participants = r.participantViewsLocked(id)
	view.MatchID = r.MatchID
	return view, nil
}

type Envelope struct {
	Type        string          `json:"type"`
	RequestID   string          `json:"requestId,omitempty"`
	RoomVersion int64           `json:"roomVersion,omitempty"`
	Payload     json.RawMessage `json:"payload,omitempty"`
}

type response struct {
	Type        string `json:"type"`
	RequestID   string `json:"requestId,omitempty"`
	RoomVersion int64  `json:"roomVersion"`
	Payload     any    `json:"payload,omitempty"`
}

func isRoomControl(command string) bool {
	switch command {
	case "room.start", "room.pause", "room.resume", "room.stop", "room.reset":
		return true
	default:
		return false
	}
}

func (r *Room) Handle(id string, envelope Envelope, now time.Time) error {
	return r.HandleContext(context.Background(), id, envelope, now)
}

func (r *Room) HandleContext(ctx context.Context, id string, envelope Envelope, now time.Time) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	participant, ok := r.Participants[id]
	if !ok {
		return errors.New("participant not found")
	}
	if participant.Spectator && envelope.Type != "seat.select" && !isRoomControl(envelope.Type) && !(envelope.Type == "settings.update" && participant.ID == r.HostID) {
		return errors.New("spectators cannot issue commands")
	}
	if r.State.Paused && !isRoomControl(envelope.Type) {
		return errors.New("room is paused")
	}
	if r.SeenRequests == nil {
		r.SeenRequests = map[string]map[string]bool{}
	}
	if envelope.RequestID != "" && r.SeenRequests[id] != nil && r.SeenRequests[id][envelope.RequestID] {
		return nil
	}
	before := r.State.Clone()
	beforeSequence := r.Sequence
	beforeMatchID := r.MatchID
	beforeRematchReady := r.RematchReady
	beforeSeenRequests := r.SeenRequests
	beforeParticipant := snapshotParticipant(participant)
	var err error
	switch envelope.Type {
	case "room.start":
		err = r.State.BeginSetup(now)
	case "room.pause":
		err = r.State.Pause(now)
	case "room.resume":
		err = r.State.Resume(now)
	case "room.stop":
		err = r.State.Stop()
	case "room.reset":
		err = r.resetRoomLocked(ctx, now)
	case "seat.select":
		var payload struct {
			Seat game.Seat `json:"seat"`
		}
		if err = json.Unmarshal(envelope.Payload, &payload); err == nil {
			err = r.selectSeatLocked(participant, payload.Seat)
		}
	case "seat.leave":
		err = r.leaveSeatLocked(participant)
	case "settings.update":
		if participant.ID != r.HostID {
			err = errors.New("only the host may change room settings")
		} else {
			var payload struct {
				Mode  game.VisibilityMode `json:"mode"`
				Clock game.ClockPreset    `json:"clock"`
			}
			if err = json.Unmarshal(envelope.Payload, &payload); err == nil {
				if r.State.Phase != game.Lobby || !payload.Mode.Valid() || !payload.Clock.Valid() {
					err = errors.New("settings can only be changed in the lobby")
				} else {
					r.State.Mode, r.State.Clock, r.State.Version = payload.Mode, payload.Clock, r.State.Version+1
				}
			}
		}
	case "setup.replace":
		var payload struct {
			Pieces map[string]game.Piece `json:"pieces"`
		}
		if err = json.Unmarshal(envelope.Payload, &payload); err == nil {
			err = r.selectDeploymentLocked(participant, payload.Pieces)
		}
	case "ready":
		var payload struct {
			Ready bool `json:"ready"`
		}
		if err = json.Unmarshal(envelope.Payload, &payload); err == nil {
			if !participant.Seat.Valid() {
				err = errors.New("select a seat first")
			} else {
				err = r.State.SetReady(r.Board, participant.Seat, payload.Ready, now)
			}
		}
	case "move":
		var payload struct{ From, To string }
		if err = json.Unmarshal(envelope.Payload, &payload); err == nil {
			_, err = r.State.Move(r.Board, participant.Seat, payload.From, payload.To, now)
		}
	case "resign":
		err = r.State.Resign(r.Board, participant.Seat, now)
	case "draw.offer":
		err = r.State.OfferDraw(participant.Seat)
	case "draw.respond":
		var payload struct {
			Accept bool `json:"accept"`
		}
		if err = json.Unmarshal(envelope.Payload, &payload); err == nil {
			err = r.State.RespondDraw(participant.Seat, payload.Accept)
		}
	case "rematch.ready":
		var payload struct {
			Ready bool `json:"ready"`
		}
		if err = json.Unmarshal(envelope.Payload, &payload); err == nil {
			if r.State.Phase != game.Finished || !participant.Seat.Valid() {
				err = errors.New("rematch is not available")
			} else {
				r.RematchReady[participant.Seat] = payload.Ready
				allReady := true
				for _, seat := range game.Seats {
					if !r.RematchReady[seat] {
						allReady = false
						break
					}
				}
				if allReady {
					err = r.resetRematchLocked(ctx, now)
				}
			}
		}
	default:
		err = fmt.Errorf("unknown command %q", envelope.Type)
	}
	if err == nil && r.State.Version == 0 {
		r.State.Version = 1
	}
	if err == nil {
		for _, current := range r.Participants {
			if persistErr := r.persistParticipantLocked(ctx, current); persistErr != nil {
				r.State = before
				r.Sequence = beforeSequence
				r.MatchID = beforeMatchID
				r.RematchReady = beforeRematchReady
				r.SeenRequests = beforeSeenRequests
				restoreParticipant(participant, beforeParticipant)
				return fmt.Errorf("persist participant: %w", persistErr)
			}
		}
		if persistErr := r.persistLocked(ctx, envelope.Type); persistErr != nil {
			r.State = before
			r.Sequence = beforeSequence
			r.MatchID = beforeMatchID
			r.RematchReady = beforeRematchReady
			r.SeenRequests = beforeSeenRequests
			restoreParticipant(participant, beforeParticipant)
			return fmt.Errorf("persist command: %w", persistErr)
		}
		if envelope.RequestID != "" {
			if r.SeenRequests[id] == nil {
				r.SeenRequests[id] = map[string]bool{}
			}
			r.SeenRequests[id][envelope.RequestID] = true
		}
	}
	return err
}

func (r *Room) selectSeatLocked(participant *Participant, seat game.Seat) error {
	if r.State.Phase != game.Lobby && r.State.Phase != game.Setup {
		return errors.New("seats are closed")
	}
	if !seat.Valid() {
		return errors.New("invalid seat")
	}
	for _, other := range r.Participants {
		if other.ID != participant.ID && !other.Spectator && other.Seat == seat {
			return errors.New("seat is already occupied")
		}
	}
	if participant.Seat.Valid() && participant.Seat != seat {
		old := participant.Seat
		r.State.Players[old] = game.Player{}
		for node, piece := range r.State.Pieces {
			if piece.Owner == old {
				delete(r.State.Pieces, node)
			}
		}
	}
	participant.Seat = seat
	participant.Spectator = false
	r.State.Players[seat] = game.Player{Username: participant.Username, Connected: participant.Connected}
	if len(r.deploymentLocked(seat)) == 0 {
		for node, piece := range game.DefaultDeployment(r.Board, seat) {
			r.State.Pieces[node] = piece
		}
	}
	r.State.Version++
	return nil
}

func (r *Room) leaveSeatLocked(participant *Participant) error {
	if r.State.Phase != game.Lobby && r.State.Phase != game.Setup {
		return errors.New("seats are closed")
	}
	if !participant.Seat.Valid() {
		return errors.New("participant is not seated")
	}
	oldSeat := participant.Seat
	participant.Seat = ""
	participant.Spectator = true
	participant.Ready = false
	r.State.Players[oldSeat] = game.Player{}
	for node, piece := range r.State.Pieces {
		if piece.Owner == oldSeat {
			delete(r.State.Pieces, node)
		}
	}
	if r.State.Phase == game.Setup {
		r.State.Phase = game.Lobby
		r.State.SetupDeadline = time.Time{}
		for seat, player := range r.State.Players {
			player.Ready = false
			r.State.Players[seat] = player
		}
	}
	r.State.Version++
	return nil
}

func (r *Room) selectDeploymentLocked(participant *Participant, pieces map[string]game.Piece) error {
	if !participant.Seat.Valid() {
		return errors.New("select a seat first")
	}
	if err := game.ValidateDeployment(r.Board, participant.Seat, pieces); err != nil {
		return err
	}
	return r.State.ReplaceDeployment(r.Board, participant.Seat, pieces)
}

func (r *Room) deploymentLocked(seat game.Seat) map[string]game.Piece {
	deployment := map[string]game.Piece{}
	for node, piece := range r.State.Pieces {
		if piece.Owner == seat {
			deployment[node] = piece
		}
	}
	return deployment
}

func (r *Room) Tick(ctx context.Context, now time.Time) bool {
	r.mu.Lock()
	defer r.mu.Unlock()
	before := r.State.Clone()
	beforeSequence := r.Sequence
	if r.State.Tick(r.Board, now) {
		r.State.Version++
		if err := r.persistLocked(ctx, "clock.timeout"); err != nil {
			r.State = before
			r.Sequence = beforeSequence
			return false
		}
		return true
	}
	return false
}

func (r *Room) persistLocked(ctx context.Context, eventType string) error {
	if r.DB == nil || r.MatchID == "" {
		return nil
	}
	for _, participant := range r.Participants {
		if !participant.Seat.Valid() || participant.ProfileID == "" {
			continue
		}
		player := r.State.Players[participant.Seat]
		if err := r.DB.UpsertMatchSeat(ctx, r.MatchID, participant.Seat.String(), participant.ProfileID, participant.Seat.Team(), player.Eliminated, player.EliminationReason); err != nil {
			return err
		}
	}
	r.Sequence++
	if err := r.DB.AppendMatchEvent(ctx, r.MatchID, r.Sequence, eventType, r.State); err != nil {
		return err
	}
	if r.Sequence == 1 || r.Sequence%10 == 0 || r.State.Phase == game.Finished {
		if err := r.DB.SaveSnapshot(ctx, r.MatchID, r.Sequence, r.State); err != nil {
			return err
		}
	}
	outcome, team := "", ""
	if r.State.Result != nil {
		outcome, team = r.State.Result.Outcome, r.State.Result.Team
	}
	if err := r.DB.UpdateMatchStatus(ctx, r.MatchID, string(r.State.Phase), outcome, team); err != nil {
		return err
	}
	return r.DB.UpdateRoom(ctx, r.PersistentID, string(r.State.Phase), string(r.State.Mode), string(r.State.Clock))
}

func (r *Room) persistParticipantLocked(ctx context.Context, participant *Participant) error {
	if r.DB == nil || r.PersistentID == "" || participant.ProfileID == "" {
		return nil
	}
	role := "player"
	if participant.Spectator {
		role = "spectator"
	}
	ready := false
	if participant.Seat.Valid() {
		ready = r.State.Players[participant.Seat].Ready
	}
	return r.DB.UpsertRoomParticipant(ctx, r.PersistentID, participant.ProfileID, role, participant.Seat.String(), ready)
}

func (r *Room) resetRematchLocked(ctx context.Context, now time.Time) error {
	opening := randomOpening()
	newMatchID := ""
	if r.DB != nil {
		matchID, err := r.DB.CreateMatch(ctx, r.PersistentID, string(r.State.Mode), string(r.State.Clock), string(opening))
		if err != nil {
			return err
		}
		newMatchID = matchID
	}
	version := r.State.Version + 1
	r.State = game.NewState(r.State.Mode, r.State.Clock, opening, now)
	r.State.Version = version
	for _, participant := range r.Participants {
		participant.Seat = ""
		participant.Ready = false
		participant.Spectator = true
	}
	r.MatchID = newMatchID
	r.Sequence = 0
	r.RematchReady = map[game.Seat]bool{}
	r.SeenRequests = map[string]map[string]bool{}
	return nil
}

func (r *Room) resetRoomLocked(ctx context.Context, now time.Time) error {
	if r.State.Phase == game.Setup || r.State.Phase == game.Playing {
		if err := r.State.Stop(); err != nil {
			return err
		}
		if err := r.persistLocked(ctx, "room.reset"); err != nil {
			return err
		}
	}

	opening := randomOpening()
	newMatchID := r.MatchID
	if r.State.Phase == game.Finished && r.DB != nil {
		matchID, err := r.DB.CreateMatch(ctx, r.PersistentID, string(r.State.Mode), string(r.State.Clock), string(opening))
		if err != nil {
			return err
		}
		newMatchID = matchID
	}
	version := r.State.Version + 1
	state := game.NewState(r.State.Mode, r.State.Clock, opening, now)
	state.Version = version
	for _, seat := range game.Seats {
		player := r.State.Players[seat]
		if player.Username == "" {
			continue
		}
		state.Players[seat] = game.Player{Username: player.Username, Connected: player.Connected}
		for node, piece := range game.DefaultDeployment(r.Board, seat) {
			state.Pieces[node] = piece
		}
	}
	for _, participant := range r.Participants {
		participant.Ready = false
		if participant.Seat.Valid() && !participant.Spectator {
			continue
		}
		participant.Seat = ""
		participant.Spectator = true
	}
	r.State = state
	r.MatchID = newMatchID
	r.Sequence = 0
	r.RematchReady = map[game.Seat]bool{}
	r.SeenRequests = map[string]map[string]bool{}
	return nil
}

func (r *Room) Broadcast(ctx context.Context) {
	r.mu.Lock()
	deferred := make([]struct {
		participant *Participant
		conn        *websocket.Conn
		view        game.View
	}, 0, len(r.Participants))
	for id, participant := range r.Participants {
		if participant.Conn == nil || !participant.Connected {
			continue
		}
		view, err := r.viewLocked(id)
		if err == nil {
			deferred = append(deferred, struct {
				participant *Participant
				conn        *websocket.Conn
				view        game.View
			}{participant, participant.Conn, view})
		}
	}
	r.mu.Unlock()
	for _, item := range deferred {
		payload, _ := json.Marshal(response{Type: "snapshot", RoomVersion: item.view.Version, Payload: item.view})
		writeCtx, cancel := context.WithTimeout(ctx, 3*time.Second)
		item.participant.writeMu.Lock()
		_ = item.conn.Write(writeCtx, websocket.MessageText, payload)
		item.participant.writeMu.Unlock()
		cancel()
	}
}

func (r *Room) viewLocked(id string) (game.View, error) {
	participant, ok := r.Participants[id]
	if !ok {
		return game.View{}, errors.New("participant not found")
	}
	view := r.State.Project(game.Viewer{Seat: participant.Seat, Spectator: participant.Spectator}, r.Board)
	view.Participants = r.participantViewsLocked(id)
	return view, nil
}

func (r *Room) SendSnapshot(ctx context.Context, id string) error {
	r.mu.Lock()
	participant, ok := r.Participants[id]
	if !ok || participant.Conn == nil {
		r.mu.Unlock()
		return errors.New("not connected")
	}
	view, err := r.viewLocked(id)
	conn := participant.Conn
	r.mu.Unlock()
	if err != nil {
		return err
	}
	payload, _ := json.Marshal(response{Type: "snapshot", RoomVersion: view.Version, Payload: view})
	participant.writeMu.Lock()
	defer participant.writeMu.Unlock()
	return conn.Write(ctx, websocket.MessageText, payload)
}

func (r *Room) SendError(ctx context.Context, id string, requestID string, err error) {
	r.mu.Lock()
	participant := r.Participants[id]
	if participant == nil || participant.Conn == nil {
		r.mu.Unlock()
		return
	}
	conn := participant.Conn
	version := r.State.Version
	r.mu.Unlock()
	payload, _ := json.Marshal(response{Type: "error", RequestID: requestID, RoomVersion: version, Payload: map[string]string{"message": err.Error()}})
	participant.writeMu.Lock()
	defer participant.writeMu.Unlock()
	_ = conn.Write(ctx, websocket.MessageText, payload)
}

func (r *Room) RunSocket(ctx context.Context, id, username, profileID string, conn *websocket.Conn) {
	if err := r.AttachOrRestore(id, username, profileID, conn); err != nil {
		return
	}
	defer r.Detach(id, conn)
	_ = r.SendSnapshot(ctx, id)
	for {
		_, data, err := conn.Read(ctx)
		if err != nil {
			return
		}
		var envelope Envelope
		if err := json.Unmarshal(data, &envelope); err != nil {
			r.SendError(ctx, id, "", errors.New("invalid realtime message"))
			continue
		}
		if err := r.HandleContext(ctx, id, envelope, time.Now().UTC()); err != nil {
			r.SendError(ctx, id, envelope.RequestID, err)
			continue
		}
		r.Broadcast(ctx)
	}
}

func randomID() string {
	var raw [16]byte
	if _, err := rand.Read(raw[:]); err != nil {
		return fmt.Sprintf("fallback-%d", time.Now().UnixNano())
	}
	return hex.EncodeToString(raw[:])
}

func NewSessionID() string { return randomID() }
