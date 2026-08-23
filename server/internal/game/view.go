package game

import "time"

type Viewer struct {
	Seat      Seat
	Spectator bool
}

type VisiblePiece struct {
	ID       string    `json:"id"`
	Owner    Seat      `json:"owner"`
	Kind     PieceKind `json:"kind,omitempty"`
	Revealed bool      `json:"revealed,omitempty"`
}

type ParticipantView struct {
	Username  string `json:"username"`
	Seat      Seat   `json:"seat,omitempty"`
	Role      string `json:"role"`
	Connected bool   `json:"connected"`
	Self      bool   `json:"self,omitempty"`
}

type View struct {
	MatchID       string                  `json:"matchId,omitempty"`
	Version       int64                   `json:"version"`
	Phase         Phase                   `json:"phase"`
	Mode          VisibilityMode          `json:"mode"`
	Clock         ClockPreset             `json:"clock"`
	Turn          Seat                    `json:"turn,omitempty"`
	Deadline      time.Time               `json:"deadline,omitempty"`
	SetupDeadline time.Time               `json:"setupDeadline,omitempty"`
	Opening       Seat                    `json:"opening"`
	Players       map[Seat]Player         `json:"players"`
	Participants  []ParticipantView       `json:"participants"`
	Pieces        map[string]VisiblePiece `json:"pieces"`
	RevealedFlags map[Seat]string         `json:"revealedFlags"`
	LegalMoves    []string                `json:"legalMoves,omitempty"`
	LastMove      *Move                   `json:"lastMove,omitempty"`
	DrawOffer     Seat                    `json:"drawOffer,omitempty"`
	Result        *Result                 `json:"result,omitempty"`
}

func (s *State) Project(viewer Viewer, board *Board) View {
	view := View{
		Version:       s.Version,
		Phase:         s.Phase,
		Mode:          s.Mode,
		Clock:         s.Clock,
		Turn:          s.Turn,
		Deadline:      s.Deadline,
		SetupDeadline: s.SetupDeadline,
		Opening:       s.Opening,
		Players:       map[Seat]Player{},
		Participants:  []ParticipantView{},
		Pieces:        map[string]VisiblePiece{},
		RevealedFlags: map[Seat]string{},
		DrawOffer:     s.DrawOffer,
	}
	for seat, player := range s.Players {
		view.Players[seat] = player
	}
	for seat, node := range s.RevealedFlags {
		view.RevealedFlags[seat] = node
	}
	for node, piece := range s.Pieces {
		visible := VisiblePiece{ID: piece.ID, Owner: piece.Owner}
		if canSeeRank(s, viewer, piece.Owner) || s.RevealedFlags[piece.Owner] == node {
			visible.Kind = piece.Kind
			visible.Revealed = true
		}
		view.Pieces[node] = visible
	}
	if s.LastMove != nil {
		move := *s.LastMove
		view.LastMove = &move
	}
	if s.Result != nil {
		result := *s.Result
		view.Result = &result
	}
	if viewer.Seat.Valid() && !viewer.Spectator {
		view.LegalMoves = s.LegalMoves(board, viewer.Seat)
	}
	return view
}

func canSeeRank(s *State, viewer Viewer, owner Seat) bool {
	if viewer.Spectator || !viewer.Seat.Valid() {
		return false
	}
	if s.Phase == Finished || s.Mode == FullyVisible {
		return true
	}
	if viewer.Seat == owner {
		return true
	}
	return s.Mode == DoubleVisible && viewer.Seat.Team() == owner.Team()
}
