package persistence

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

type DB struct {
	Pool *pgxpool.Pool
}

type Identity struct {
	ProfileID string
	Username  string
}

type PersistedRoom struct {
	ID            string
	InviteCode    string
	Phase         string
	HostProfileID string
	Mode          string
	Clock         string
	Opening       string
	MatchID       string
	Sequence      int64
	State         []byte
}

type PersistedParticipant struct {
	ProfileID string
	Username  string
	Role      string
	Seat      string
	Ready     bool
}

type ReplayEvent struct {
	Sequence  int64           `json:"sequence"`
	Type      string          `json:"type"`
	Payload   json.RawMessage `json:"payload"`
	CreatedAt time.Time       `json:"createdAt"`
}

type SavedLayout struct {
	ID             string          `json:"id"`
	Name           string          `json:"name"`
	RulesetVersion string          `json:"rulesetVersion"`
	BoardVersion   string          `json:"boardVersion"`
	Deployment     json.RawMessage `json:"deployment"`
	timeUpdated    time.Time
}

type ProfileSummary struct {
	ID       string `json:"id"`
	Username string `json:"username"`
	Matches  int64  `json:"matches"`
	Wins     int64  `json:"wins"`
	Losses   int64  `json:"losses"`
	Draws    int64  `json:"draws"`
}

type MatchSummary struct {
	ID       string     `json:"id"`
	Outcome  string     `json:"outcome"`
	Mode     string     `json:"mode"`
	Clock    string     `json:"clock"`
	Started  *time.Time `json:"startedAt,omitempty"`
	Finished *time.Time `json:"finishedAt,omitempty"`
}

func (db *DB) CreateRoom(ctx context.Context, inviteCode, profileID string, mode, clock string) (string, error) {
	if db == nil || db.Pool == nil {
		return "", nil
	}
	var roomID string
	err := db.Pool.QueryRow(ctx, `
		INSERT INTO rooms (invite_code, phase, host_profile_id, visibility_mode, clock_preset)
		VALUES ($1, 'lobby', $2::uuid, $3, $4)
		RETURNING id::text
	`, inviteCode, profileID, mode, clock).Scan(&roomID)
	return roomID, err
}

func (db *DB) CreateMatch(ctx context.Context, roomID, mode, clock, opening string) (string, error) {
	if db == nil || db.Pool == nil {
		return "", nil
	}
	var matchID string
	err := db.Pool.QueryRow(ctx, `
		INSERT INTO matches (room_id, visibility_mode, clock_preset, opening_seat)
		VALUES ($1::uuid, $2, $3, $4)
		RETURNING id::text
	`, roomID, mode, clock, opening).Scan(&matchID)
	return matchID, err
}

func (db *DB) UpdateRoom(ctx context.Context, roomID, phase, mode, clock string) error {
	if db == nil || db.Pool == nil {
		return nil
	}
	_, err := db.Pool.Exec(ctx, `
		UPDATE rooms SET phase = $2, visibility_mode = $3, clock_preset = $4, updated_at = now()
		WHERE id = $1::uuid
	`, roomID, phase, mode, clock)
	return err
}

func (db *DB) UpdateMatchStatus(ctx context.Context, matchID, phase, outcome, team string) error {
	if db == nil || db.Pool == nil || matchID == "" {
		return nil
	}
	if phase == "playing" {
		_, err := db.Pool.Exec(ctx, `UPDATE matches SET started_at = COALESCE(started_at, now()), updated_at = now() WHERE id = $1::uuid`, matchID)
		return err
	}
	if phase == "finished" {
		_, err := db.Pool.Exec(ctx, `UPDATE matches SET outcome = $2, finished_at = COALESCE(finished_at, now()), updated_at = now() WHERE id = $1::uuid`, matchID, outcome+":"+team)
		return err
	}
	return nil
}

func (db *DB) UpsertMatchSeat(ctx context.Context, matchID, seat, profileID, team string, eliminated bool, reason string) error {
	if db == nil || db.Pool == nil || matchID == "" || profileID == "" {
		return nil
	}
	_, err := db.Pool.Exec(ctx, `
		INSERT INTO match_seats (match_id, seat, profile_id, team, eliminated, elimination_reason)
		VALUES ($1::uuid, $2, $3::uuid, $4, $5, $6)
		ON CONFLICT (match_id, seat) DO UPDATE SET profile_id = EXCLUDED.profile_id,
		team = EXCLUDED.team, eliminated = EXCLUDED.eliminated, elimination_reason = EXCLUDED.elimination_reason
	`, matchID, seat, profileID, team, eliminated, reason)
	return err
}

func (db *DB) GetProfileSummary(ctx context.Context, username string) (ProfileSummary, error) {
	if db == nil || db.Pool == nil {
		return ProfileSummary{}, fmt.Errorf("profile not found")
	}
	var summary ProfileSummary
	err := db.Pool.QueryRow(ctx, `
		SELECT p.id::text, p.username,
		       COUNT(DISTINCT m.id) FILTER (WHERE m.outcome IS NOT NULL),
		       COUNT(DISTINCT m.id) FILTER (WHERE split_part(m.outcome, ':', 1) = 'win' AND split_part(m.outcome, ':', 2) = ms.team),
		       COUNT(DISTINCT m.id) FILTER (WHERE split_part(m.outcome, ':', 1) = 'win' AND split_part(m.outcome, ':', 2) <> ms.team),
		       COUNT(DISTINCT m.id) FILTER (WHERE split_part(m.outcome, ':', 1) = 'draw')
		FROM profiles p
		LEFT JOIN match_seats ms ON ms.profile_id = p.id
		LEFT JOIN matches m ON m.id = ms.match_id
		WHERE p.username = $1
		GROUP BY p.id, p.username
	`, username).Scan(&summary.ID, &summary.Username, &summary.Matches, &summary.Wins, &summary.Losses, &summary.Draws)
	return summary, err
}

func (db *DB) ListProfileMatches(ctx context.Context, username string) ([]MatchSummary, error) {
	if db == nil || db.Pool == nil {
		return []MatchSummary{}, nil
	}
	rows, err := db.Pool.Query(ctx, `
		SELECT DISTINCT m.id::text, COALESCE(m.outcome, 'active'), m.visibility_mode,
		       m.clock_preset, m.started_at, m.finished_at
		FROM matches m JOIN match_seats ms ON ms.match_id = m.id
		JOIN profiles p ON p.id = ms.profile_id
		WHERE p.username = $1
		ORDER BY COALESCE(m.finished_at, m.started_at) DESC NULLS LAST
	`, username)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	matches := []MatchSummary{}
	for rows.Next() {
		var match MatchSummary
		if err := rows.Scan(&match.ID, &match.Outcome, &match.Mode, &match.Clock, &match.Started, &match.Finished); err != nil {
			return nil, err
		}
		matches = append(matches, match)
	}
	return matches, rows.Err()
}

func (db *DB) UpsertRoomParticipant(ctx context.Context, roomID, profileID, role, seat string, ready bool) error {
	if db == nil || db.Pool == nil || roomID == "" || profileID == "" {
		return nil
	}
	var seatValue any
	if seat != "" {
		seatValue = seat
	}
	_, err := db.Pool.Exec(ctx, `
		INSERT INTO room_participants (room_id, profile_id, role, seat, connected_at, ready)
		VALUES ($1::uuid, $2::uuid, $3, $4, now(), $5)
		ON CONFLICT (room_id, profile_id) DO UPDATE SET role = EXCLUDED.role,
		seat = EXCLUDED.seat, connected_at = EXCLUDED.connected_at, ready = EXCLUDED.ready
	`, roomID, profileID, role, seatValue, ready)
	return err
}

func (db *DB) LoadActiveRooms(ctx context.Context) ([]PersistedRoom, error) {
	if db == nil || db.Pool == nil {
		return nil, nil
	}
	rows, err := db.Pool.Query(ctx, `
		SELECT r.id::text, r.invite_code, r.phase, COALESCE(r.host_profile_id::text, ''),
		       r.visibility_mode, r.clock_preset, m.opening_seat, m.id::text,
		       COALESCE(e.sequence, 0), COALESCE(e.payload, '{}'::jsonb)
		FROM rooms r
		LEFT JOIN LATERAL (
			SELECT id, opening_seat FROM matches WHERE room_id = r.id ORDER BY updated_at DESC LIMIT 1
		) m ON true
		LEFT JOIN LATERAL (
			SELECT sequence, payload FROM match_events WHERE match_id = m.id ORDER BY sequence DESC LIMIT 1
		) e ON true
		WHERE r.phase <> 'finished'
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	rooms := []PersistedRoom{}
	for rows.Next() {
		var room PersistedRoom
		if err := rows.Scan(&room.ID, &room.InviteCode, &room.Phase, &room.HostProfileID, &room.Mode, &room.Clock, &room.Opening, &room.MatchID, &room.Sequence, &room.State); err != nil {
			return nil, err
		}
		rooms = append(rooms, room)
	}
	return rooms, rows.Err()
}

func (db *DB) LoadRoomParticipants(ctx context.Context, roomID string) ([]PersistedParticipant, error) {
	if db == nil || db.Pool == nil {
		return nil, nil
	}
	rows, err := db.Pool.Query(ctx, `
		SELECT p.id::text, p.username, rp.role, COALESCE(rp.seat, ''), rp.ready
		FROM room_participants rp JOIN profiles p ON p.id = rp.profile_id
		WHERE rp.room_id = $1::uuid
		ORDER BY rp.connected_at NULLS LAST, p.username
	`, roomID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	participants := []PersistedParticipant{}
	for rows.Next() {
		var participant PersistedParticipant
		if err := rows.Scan(&participant.ProfileID, &participant.Username, &participant.Role, &participant.Seat, &participant.Ready); err != nil {
			return nil, err
		}
		participants = append(participants, participant)
	}
	return participants, rows.Err()
}

func (db *DB) LoadReplay(ctx context.Context, matchID string) ([]ReplayEvent, error) {
	if db == nil || db.Pool == nil {
		return nil, fmt.Errorf("replay not found")
	}
	var finished bool
	if err := db.Pool.QueryRow(ctx, `SELECT outcome IS NOT NULL FROM matches WHERE id = $1::uuid`, matchID).Scan(&finished); err != nil || !finished {
		return nil, fmt.Errorf("replay not found")
	}
	rows, err := db.Pool.Query(ctx, `
		SELECT sequence, event_type, payload, created_at
		FROM match_events WHERE match_id = $1::uuid ORDER BY sequence
	`, matchID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	events := []ReplayEvent{}
	for rows.Next() {
		var event ReplayEvent
		if err := rows.Scan(&event.Sequence, &event.Type, &event.Payload, &event.CreatedAt); err != nil {
			return nil, err
		}
		events = append(events, event)
	}
	return events, rows.Err()
}

func (db *DB) ListSavedLayouts(ctx context.Context, profileID string) ([]SavedLayout, error) {
	if db == nil || db.Pool == nil {
		return []SavedLayout{}, nil
	}
	rows, err := db.Pool.Query(ctx, `
		SELECT id::text, name, ruleset_version, board_version, deployment
		FROM saved_layouts WHERE profile_id = $1::uuid ORDER BY updated_at DESC, name
	`, profileID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	layouts := []SavedLayout{}
	for rows.Next() {
		var layout SavedLayout
		if err := rows.Scan(&layout.ID, &layout.Name, &layout.RulesetVersion, &layout.BoardVersion, &layout.Deployment); err != nil {
			return nil, err
		}
		layouts = append(layouts, layout)
	}
	return layouts, rows.Err()
}

func (db *DB) CreateSavedLayout(ctx context.Context, profileID, name string, deployment json.RawMessage) (SavedLayout, error) {
	if db == nil || db.Pool == nil {
		return SavedLayout{}, fmt.Errorf("database is not configured")
	}
	var layout SavedLayout
	err := db.Pool.QueryRow(ctx, `
		INSERT INTO saved_layouts (profile_id, name, deployment)
		VALUES ($1::uuid, $2, $3::jsonb)
		RETURNING id::text, name, ruleset_version, board_version, deployment
	`, profileID, name, deployment).Scan(&layout.ID, &layout.Name, &layout.RulesetVersion, &layout.BoardVersion, &layout.Deployment)
	return layout, err
}

func (db *DB) UpdateSavedLayout(ctx context.Context, profileID, layoutID, name string, deployment json.RawMessage) (SavedLayout, error) {
	if db == nil || db.Pool == nil {
		return SavedLayout{}, fmt.Errorf("database is not configured")
	}
	var layout SavedLayout
	err := db.Pool.QueryRow(ctx, `
		UPDATE saved_layouts SET name = $3, deployment = $4::jsonb, updated_at = now()
		WHERE id = $1::uuid AND profile_id = $2::uuid
		RETURNING id::text, name, ruleset_version, board_version, deployment
	`, layoutID, profileID, name, deployment).Scan(&layout.ID, &layout.Name, &layout.RulesetVersion, &layout.BoardVersion, &layout.Deployment)
	return layout, err
}

func (db *DB) DeleteSavedLayout(ctx context.Context, profileID, layoutID string) error {
	if db == nil || db.Pool == nil {
		return fmt.Errorf("database is not configured")
	}
	result, err := db.Pool.Exec(ctx, `DELETE FROM saved_layouts WHERE id = $1::uuid AND profile_id = $2::uuid`, layoutID, profileID)
	if err != nil {
		return err
	}
	if result.RowsAffected() == 0 {
		return fmt.Errorf("layout not found")
	}
	return nil
}

func Open(ctx context.Context, dsn string) (*DB, error) {
	if dsn == "" {
		return nil, nil
	}
	config, err := pgxpool.ParseConfig(dsn)
	if err != nil {
		return nil, fmt.Errorf("parse database URL: %w", err)
	}
	config.MaxConns = 10
	config.MinConns = 1
	pool, err := pgxpool.NewWithConfig(ctx, config)
	if err != nil {
		return nil, fmt.Errorf("open database pool: %w", err)
	}
	return &DB{Pool: pool}, nil
}

func (db *DB) Close() {
	if db != nil && db.Pool != nil {
		db.Pool.Close()
	}
}

func (db *DB) Ping(ctx context.Context) error {
	if db == nil || db.Pool == nil {
		return nil
	}
	return db.Pool.Ping(ctx)
}

func tokenHash(token string) []byte {
	hash := sha256.Sum256([]byte(token))
	return hash[:]
}

func (db *DB) CreateSession(ctx context.Context, token, username string) (Identity, error) {
	if db == nil || db.Pool == nil {
		return Identity{Username: username}, nil
	}
	var identity Identity
	err := db.Pool.QueryRow(ctx, `
		INSERT INTO profiles (username) VALUES ($1)
		ON CONFLICT (username) DO UPDATE SET updated_at = now()
		RETURNING id::text, username
	`, username).Scan(&identity.ProfileID, &identity.Username)
	if err != nil {
		return Identity{}, err
	}
	_, err = db.Pool.Exec(ctx, `
		INSERT INTO sessions (token_hash, profile_id, expires_at)
		VALUES ($1, $2::uuid, now() + interval '30 days')
	`, tokenHash(token), identity.ProfileID)
	return identity, err
}

func (db *DB) LookupSession(ctx context.Context, token string) (Identity, error) {
	if db == nil || db.Pool == nil {
		return Identity{}, fmt.Errorf("database is not configured")
	}
	var identity Identity
	err := db.Pool.QueryRow(ctx, `
		SELECT p.id::text, p.username
		FROM sessions s JOIN profiles p ON p.id = s.profile_id
		WHERE s.token_hash = $1 AND s.expires_at > now()
	`, tokenHash(token)).Scan(&identity.ProfileID, &identity.Username)
	if err == nil {
		_, _ = db.Pool.Exec(ctx, `UPDATE sessions SET last_seen_at = now() WHERE token_hash = $1`, tokenHash(token))
	}
	return identity, err
}

func (db *DB) DeleteSession(ctx context.Context, token string) error {
	if db == nil || db.Pool == nil {
		return nil
	}
	_, err := db.Pool.Exec(ctx, `DELETE FROM sessions WHERE token_hash = $1`, tokenHash(token))
	return err
}

func (db *DB) AppendMatchEvent(ctx context.Context, matchID string, sequence int64, eventType string, payload any) error {
	if db == nil || db.Pool == nil {
		return nil
	}
	raw, err := json.Marshal(payload)
	if err != nil {
		return err
	}
	_, err = db.Pool.Exec(ctx, `
		INSERT INTO match_events (match_id, sequence, event_type, payload)
		VALUES ($1, $2, $3, $4::jsonb)
	`, matchID, sequence, eventType, raw)
	return err
}

func (db *DB) SaveSnapshot(ctx context.Context, matchID string, sequence int64, state any) error {
	if db == nil || db.Pool == nil {
		return nil
	}
	raw, err := json.Marshal(state)
	if err != nil {
		return err
	}
	_, err = db.Pool.Exec(ctx, `
		INSERT INTO match_snapshots (match_id, sequence, state)
		VALUES ($1, $2, $3::jsonb)
		ON CONFLICT (match_id, sequence) DO UPDATE SET state = EXCLUDED.state
	`, matchID, sequence, raw)
	return err
}

func (db *DB) Touch(ctx context.Context, matchID string) error {
	if db == nil || db.Pool == nil {
		return nil
	}
	_, err := db.Pool.Exec(ctx, `UPDATE matches SET updated_at = $2 WHERE id = $1`, matchID, time.Now().UTC())
	return err
}
