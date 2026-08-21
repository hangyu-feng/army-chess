package game

import (
	"errors"
	"fmt"
	"sort"
	"time"
)

type Seat string

const (
	North Seat = "north"
	East  Seat = "east"
	South Seat = "south"
	West  Seat = "west"
)

var Seats = []Seat{North, East, South, West}

func (s Seat) Valid() bool {
	return s == North || s == East || s == South || s == West
}

func (s Seat) Team() string {
	if s == North || s == South {
		return "north_south"
	}
	return "east_west"
}

func (s Seat) String() string { return string(s) }

func nextSeat(from Seat, eliminated map[Seat]bool) Seat {
	for i, seat := range Seats {
		if seat != from {
			continue
		}
		for step := 1; step <= len(Seats); step++ {
			candidate := Seats[(i+step)%len(Seats)]
			if !eliminated[candidate] {
				return candidate
			}
		}
	}
	return ""
}

type Phase string

const (
	Lobby    Phase = "lobby"
	Setup    Phase = "setup"
	Playing  Phase = "playing"
	Finished Phase = "finished"
)

func (p Phase) Valid() bool {
	return p == Lobby || p == Setup || p == Playing || p == Finished
}

type VisibilityMode string

const (
	FourDark      VisibilityMode = "four_dark"
	DoubleVisible VisibilityMode = "double_visible"
	FullyVisible  VisibilityMode = "fully_visible"
)

func (m VisibilityMode) Valid() bool {
	return m == FourDark || m == DoubleVisible || m == FullyVisible
}

type ClockPreset string

const (
	Fast     ClockPreset = "fast"
	Standard ClockPreset = "standard"
	Relaxed  ClockPreset = "relaxed"
)

func (c ClockPreset) Valid() bool {
	return c == Fast || c == Standard || c == Relaxed
}

func (c ClockPreset) TurnDuration() time.Duration {
	switch c {
	case Fast:
		return 20 * time.Second
	case Relaxed:
		return 120 * time.Second
	default:
		return 60 * time.Second
	}
}

func (c ClockPreset) SetupDuration() time.Duration {
	switch c {
	case Fast:
		return 60 * time.Second
	case Relaxed:
		return 300 * time.Second
	default:
		return 120 * time.Second
	}
}

type PieceKind string

const (
	Flag      PieceKind = "flag"
	Commander PieceKind = "commander"
	Marshal   PieceKind = "marshal"
	Division  PieceKind = "division"
	Brigade   PieceKind = "brigade"
	Regiment  PieceKind = "regiment"
	Battalion PieceKind = "battalion"
	Bomb      PieceKind = "bomb"
	Company   PieceKind = "company"
	Platoon   PieceKind = "platoon"
	Engineer  PieceKind = "engineer"
	Mine      PieceKind = "mine"
)

var pieceInventory = []PieceKind{
	Flag, Commander, Marshal,
	Division, Division, Brigade, Brigade, Regiment, Regiment, Battalion, Battalion,
	Bomb, Bomb, Company, Company, Company, Platoon, Platoon, Platoon,
	Engineer, Engineer, Engineer, Mine, Mine, Mine,
}

var rankValues = map[PieceKind]int{
	Engineer: 1, Platoon: 2, Company: 3, Battalion: 4,
	Regiment: 5, Brigade: 6, Division: 7, Marshal: 8,
	Commander: 9,
}

func Inventory() []PieceKind { return append([]PieceKind(nil), pieceInventory...) }

func (k PieceKind) Rank() int { return rankValues[k] }

func (k PieceKind) Immobile() bool { return k == Flag || k == Mine }

type Piece struct {
	ID    string    `json:"id"`
	Owner Seat      `json:"owner"`
	Kind  PieceKind `json:"kind"`
}

type Player struct {
	Username          string `json:"username"`
	Ready             bool   `json:"ready"`
	Connected         bool   `json:"connected"`
	Eliminated        bool   `json:"eliminated"`
	EliminationReason string `json:"eliminationReason,omitempty"`
	Misses            int    `json:"misses"`
}

type Move struct {
	Seat   Seat   `json:"seat"`
	From   string `json:"from"`
	To     string `json:"to"`
	Result string `json:"result"`
}

type Result struct {
	Outcome string `json:"outcome"`
	Team    string `json:"team,omitempty"`
	Reason  string `json:"reason,omitempty"`
}

type State struct {
	Version        int64            `json:"version"`
	Phase          Phase            `json:"phase"`
	Mode           VisibilityMode   `json:"mode"`
	Clock          ClockPreset      `json:"clock"`
	Opening        Seat             `json:"opening"`
	Turn           Seat             `json:"turn"`
	Deadline       time.Time        `json:"deadline,omitempty"`
	SetupDeadline  time.Time        `json:"setupDeadline,omitempty"`
	Players        map[Seat]Player  `json:"players"`
	Pieces         map[string]Piece `json:"pieces"`
	RevealedFlags  map[Seat]string  `json:"revealedFlags"`
	NoCaptureMoves int              `json:"noCaptureMoves"`
	DrawOffer      Seat             `json:"drawOffer,omitempty"`
	DrawAccepts    map[Seat]bool    `json:"drawAccepts,omitempty"`
	LastMove       *Move            `json:"lastMove,omitempty"`
	Result         *Result          `json:"result,omitempty"`
}

func NewState(mode VisibilityMode, clock ClockPreset, opening Seat, now time.Time) *State {
	if !opening.Valid() {
		opening = North
	}
	s := &State{
		Version:       1,
		Phase:         Lobby,
		Mode:          mode,
		Clock:         clock,
		Opening:       opening,
		Players:       map[Seat]Player{},
		Pieces:        map[string]Piece{},
		RevealedFlags: map[Seat]string{},
		DrawAccepts:   map[Seat]bool{},
	}
	for _, seat := range Seats {
		s.Players[seat] = Player{}
	}
	return s
}

func (s *State) Clone() *State {
	copyState := *s
	copyState.Players = map[Seat]Player{}
	for seat, player := range s.Players {
		copyState.Players[seat] = player
	}
	copyState.Pieces = map[string]Piece{}
	for node, piece := range s.Pieces {
		copyState.Pieces[node] = piece
	}
	copyState.RevealedFlags = map[Seat]string{}
	for seat, node := range s.RevealedFlags {
		copyState.RevealedFlags[seat] = node
	}
	copyState.DrawAccepts = map[Seat]bool{}
	for seat, accepted := range s.DrawAccepts {
		copyState.DrawAccepts[seat] = accepted
	}
	if s.LastMove != nil {
		move := *s.LastMove
		copyState.LastMove = &move
	}
	if s.Result != nil {
		result := *s.Result
		copyState.Result = &result
	}
	return &copyState
}

func (s *State) ActiveSeats() []Seat {
	active := make([]Seat, 0, 4)
	for _, seat := range Seats {
		if !s.Players[seat].Eliminated {
			active = append(active, seat)
		}
	}
	return active
}

func (s *State) Validate() error {
	if !s.Phase.Valid() {
		return errors.New("invalid phase")
	}
	if !s.Mode.Valid() {
		return errors.New("invalid visibility mode")
	}
	if !s.Clock.Valid() {
		return errors.New("invalid clock preset")
	}
	seen := map[string]bool{}
	for node, piece := range s.Pieces {
		if !piece.Owner.Valid() || piece.ID == "" || piece.Kind == "" {
			return fmt.Errorf("invalid piece at %s", node)
		}
		if seen[piece.ID] {
			return fmt.Errorf("duplicate piece %s", piece.ID)
		}
		seen[piece.ID] = true
	}
	return nil
}

func sortedNodes(pieces map[string]Piece, owner Seat) []string {
	nodes := make([]string, 0)
	for node, piece := range pieces {
		if piece.Owner == owner {
			nodes = append(nodes, node)
		}
	}
	sort.Strings(nodes)
	return nodes
}
