package game

import (
	"errors"
	"fmt"
	"sort"
	"time"
)

var (
	ErrNotPlaying   = errors.New("match is not in progress")
	ErrNotYourTurn  = errors.New("it is not your turn")
	ErrIllegalMove  = errors.New("illegal move")
	ErrFriendlyFire = errors.New("allied pieces cannot attack")
	ErrInvalidSetup = errors.New("deployment is invalid")
	ErrAlreadyReady = errors.New("player is already ready")
	ErrNotReady     = errors.New("all four players must be ready")
)

func DefaultDeployment(board *Board, seat Seat) map[string]Piece {
	nodes := board.DeploymentNodes(seat)
	deployment := map[string]Piece{}
	used := map[string]bool{}
	index := 1
	for _, node := range nodes {
		if board.Nodes[node].Type == Headquarters {
			deployment[node] = Piece{ID: fmt.Sprintf("%s-%02d", seat, index), Owner: seat, Kind: Flag}
			used[node] = true
			index++
			break
		}
	}
	for _, kind := range []PieceKind{Mine, Mine, Mine} {
		for _, node := range nodes {
			if used[node] || !backTwoRows(board, seat, node) {
				continue
			}
			deployment[node] = Piece{ID: fmt.Sprintf("%s-%02d", seat, index), Owner: seat, Kind: kind}
			used[node] = true
			index++
			break
		}
	}
	remaining := make([]PieceKind, 0, len(pieceInventory)-4)
	minesPlaced := 0
	flagPlaced := false
	for _, kind := range pieceInventory {
		if kind == Flag && !flagPlaced {
			flagPlaced = true
			continue
		}
		if kind == Mine && minesPlaced < 3 {
			minesPlaced++
			continue
		}
		remaining = append(remaining, kind)
	}
	for _, kind := range remaining {
		for _, node := range nodes {
			if !used[node] && !(kind == Bomb && frontRow(board, seat, node)) {
				deployment[node] = Piece{ID: fmt.Sprintf("%s-%02d", seat, index), Owner: seat, Kind: kind}
				used[node] = true
				index++
				break
			}
		}
	}
	return deployment
}

func (s *State) ReplaceDeployment(board *Board, seat Seat, deployment map[string]Piece) error {
	if s.Phase != Lobby && s.Phase != Setup {
		return errors.New("deployment is closed")
	}
	if err := ValidateDeployment(board, seat, deployment); err != nil {
		return err
	}
	for node, piece := range s.Pieces {
		if piece.Owner == seat {
			delete(s.Pieces, node)
		}
	}
	for node, piece := range deployment {
		s.Pieces[node] = piece
	}
	s.Version++
	return nil
}

func ValidateDeployment(board *Board, seat Seat, deployment map[string]Piece) error {
	if len(deployment) != len(pieceInventory) {
		return fmt.Errorf("deployment needs %d pieces", len(pieceInventory))
	}
	allowed := map[string]bool{}
	for _, node := range board.DeploymentNodes(seat) {
		allowed[node] = true
	}
	counts := map[PieceKind]int{}
	for node, piece := range deployment {
		if !allowed[node] || piece.Owner != seat {
			return fmt.Errorf("piece is not in a deployment position: %s", node)
		}
		counts[piece.Kind]++
	}
	for _, kind := range pieceInventory {
		if counts[kind] == 0 {
			return fmt.Errorf("missing %s", kind)
		}
		counts[kind]--
	}
	for kind, count := range counts {
		if count != 0 {
			return fmt.Errorf("wrong count for %s", kind)
		}
	}
	flagInHQ := false
	for node, piece := range deployment {
		if piece.Kind == Flag {
			n, _ := board.Node(node)
			flagInHQ = n.Type == Headquarters
		}
		if piece.Kind == Bomb {
			if frontRow(board, seat, node) {
				return errors.New("bomb cannot be placed in the front row")
			}
		}
		if piece.Kind == Mine && !backTwoRows(board, seat, node) {
			return errors.New("mine must be placed in the final two rows")
		}
	}
	if !flagInHQ {
		return errors.New("flag must be in headquarters")
	}
	return nil
}

func (s *State) Start(board *Board, now time.Time) error {
	if s.Phase != Lobby && s.Phase != Setup {
		return errors.New("match has already started")
	}
	for _, seat := range Seats {
		player := s.Players[seat]
		if player.Username == "" {
			return ErrNotReady
		}
		deployment := map[string]Piece{}
		for node, piece := range s.Pieces {
			if piece.Owner == seat {
				deployment[node] = piece
			}
		}
		if err := ValidateDeployment(board, seat, deployment); err != nil {
			return fmt.Errorf("%s: %w", seat, err)
		}
		if !player.Ready {
			return ErrNotReady
		}
	}
	s.Phase = Playing
	s.Turn = s.Opening
	s.Deadline = now.Add(s.Clock.TurnDuration())
	s.SetupDeadline = time.Time{}
	s.Version++
	if len(s.LegalMoves(board, s.Turn)) == 0 {
		s.eliminate(s.Turn, "no_legal_move")
		if s.Phase == Playing {
			s.advanceTurnWithBoard(board, now)
		}
	}
	return nil
}

func (s *State) SetReady(board *Board, seat Seat, ready bool, now time.Time) error {
	if s.Phase == Lobby {
		for _, required := range Seats {
			if s.Players[required].Username == "" {
				return ErrNotReady
			}
		}
		s.Phase = Setup
		s.SetupDeadline = now.Add(s.Clock.SetupDuration())
	}
	if s.Phase != Setup {
		return errors.New("ready state is closed")
	}
	if s.Players[seat].Username == "" {
		return errors.New("seat is empty")
	}
	if ready {
		deployment := map[string]Piece{}
		for node, piece := range s.Pieces {
			if piece.Owner == seat {
				deployment[node] = piece
			}
		}
		if err := ValidateDeployment(board, seat, deployment); err != nil {
			return err
		}
	}
	player := s.Players[seat]
	player.Ready = ready
	s.Players[seat] = player
	s.Version++
	if ready {
		allReady := true
		for _, current := range Seats {
			if !s.Players[current].Ready {
				allReady = false
			}
		}
		if allReady {
			return s.Start(board, now)
		}
	}
	return nil
}

func (s *State) LegalMoves(board *Board, seat Seat) []string {
	if s.Phase != Playing || s.Players[seat].Eliminated {
		return nil
	}
	result := []string{}
	for from, piece := range s.Pieces {
		if piece.Owner != seat || piece.Kind.Immobile() {
			continue
		}
		for to := range s.legalDestinations(board, from, piece) {
			result = append(result, from+"->"+to)
		}
	}
	sort.Strings(result)
	return result
}

func (s *State) legalDestinations(board *Board, from string, piece Piece) map[string]bool {
	result := map[string]bool{}
	if node, ok := board.Node(from); ok && node.Type == Headquarters {
		return result
	}
	for _, edge := range board.Adj[from] {
		if occupant, ok := s.Pieces[edge.To]; ok && occupant.Owner.Team() == piece.Owner.Team() {
			continue
		}
		if occupant, ok := s.Pieces[edge.To]; ok && board.Nodes[edge.To].Type == Camp && occupant.Owner != piece.Owner {
			continue
		}
		if board.Nodes[edge.To].Type == Camp {
			result[edge.To] = true
			continue
		}
		result[edge.To] = true
	}
	// Engineers can use the full connected railway path, including turns.
	if piece.Kind == Engineer {
		queue := []string{from}
		seen := map[string]bool{from: true}
		for len(queue) > 0 {
			current := queue[0]
			queue = queue[1:]
			for _, edge := range board.Adj[current] {
				if edge.Type != "rail" || seen[edge.To] {
					continue
				}
				if occupant, ok := s.Pieces[edge.To]; ok {
					if occupant.Owner.Team() == piece.Owner.Team() {
						continue
					}
					if board.Nodes[edge.To].Type != Camp {
						result[edge.To] = true
					}
					continue
				}
				seen[edge.To] = true
				queue = append(queue, edge.To)
				result[edge.To] = true
			}
		}
	} else {
		fromNode, ok := board.Node(from)
		if !ok {
			return result
		}
		for _, direction := range [][2]int{{1, 0}, {-1, 0}, {0, 1}, {0, -1}} {
			x, y := int(fromNode.X*float64(board.Width-1)), int(fromNode.Y*float64(board.Height-1))
			for {
				x += direction[0]
				y += direction[1]
				if x < 0 || y < 0 || x >= board.Width || y >= board.Height {
					break
				}
				node := fmt.Sprintf("n%02d_%02d", x, y)
				if occupant, ok := s.Pieces[node]; ok {
					if occupant.Owner.Team() != piece.Owner.Team() && board.Nodes[node].Type != Camp {
						result[node] = true
					}
					break
				}
				if board.Nodes[node].Type != Camp {
					result[node] = true
				}
			}
		}
	}
	return result
}

func frontRow(board *Board, seat Seat, nodeID string) bool {
	node, ok := board.Node(nodeID)
	if !ok {
		return false
	}
	const edge = 0.0001
	switch seat {
	case North:
		return node.Y >= 4.0/11.0-edge
	case East:
		return node.X <= 7.0/11.0+edge
	case South:
		return node.Y <= 7.0/11.0+edge
	case West:
		return node.X >= 4.0/11.0-edge
	default:
		return false
	}
}

func backTwoRows(board *Board, seat Seat, nodeID string) bool {
	node, ok := board.Node(nodeID)
	if !ok {
		return false
	}
	const edge = 0.0001
	switch seat {
	case North:
		return node.Y <= 1.0/11.0+edge
	case East:
		return node.X >= 10.0/11.0-edge
	case South:
		return node.Y >= 10.0/11.0-edge
	case West:
		return node.X <= 1.0/11.0+edge
	default:
		return false
	}
}

func (s *State) Move(board *Board, seat Seat, from, to string, now time.Time) (string, error) {
	if s.Phase != Playing {
		return "", ErrNotPlaying
	}
	if s.Turn != seat {
		return "", ErrNotYourTurn
	}
	piece, ok := s.Pieces[from]
	if !ok || piece.Owner != seat || piece.Kind.Immobile() {
		return "", ErrIllegalMove
	}
	destinations := s.legalDestinations(board, from, piece)
	if !destinations[to] {
		return "", ErrIllegalMove
	}
	target, occupied := s.Pieces[to]
	if occupied && target.Owner.Team() == piece.Owner.Team() {
		return "", ErrFriendlyFire
	}
	delete(s.Pieces, from)
	result := "move"
	removed := false
	if !occupied {
		s.Pieces[to] = piece
	} else {
		winner, combatResult := resolveCombat(piece, target)
		result = combatResult
		removed = true
		delete(s.Pieces, to)
		if winner != nil {
			s.Pieces[to] = *winner
		}
		if target.Kind == Flag {
			s.RevealedFlags[target.Owner] = to
			s.eliminate(target.Owner, "flag_captured")
		}
		if piece.Kind == Commander {
			s.revealFlag(piece.Owner)
		}
		if target.Kind == Commander {
			s.revealFlag(target.Owner)
		}
	}
	s.LastMove = &Move{Seat: seat, From: from, To: to, Result: result}
	if removed {
		s.NoCaptureMoves = 0
	} else {
		s.NoCaptureMoves++
	}
	s.DrawOffer = ""
	s.DrawAccepts = map[Seat]bool{}
	s.Version++
	if s.NoCaptureMoves >= 70 {
		s.finish("draw", "seventy_moves")
		return result, nil
	}
	if s.Phase == Finished {
		return result, nil
	}
	s.advanceTurnWithBoard(board, now)
	return result, nil
}

func resolveCombat(attacker, defender Piece) (*Piece, string) {
	if attacker.Kind == Bomb || defender.Kind == Bomb {
		return nil, "both_removed"
	}
	if defender.Kind == Mine {
		if attacker.Kind == Engineer {
			return &attacker, "engineer_cleared_mine"
		}
		return &defender, "mine_survives"
	}
	if attacker.Kind == Mine {
		return nil, "mine_removed_attacker"
	}
	if attacker.Kind.Rank() > defender.Kind.Rank() {
		return &attacker, "attacker_survives"
	}
	if attacker.Kind.Rank() < defender.Kind.Rank() {
		return &defender, "defender_survives"
	}
	return nil, "both_removed"
}

func (s *State) AdvanceTurn(board *Board, now time.Time) {
	s.advanceTurnWithBoard(board, now)
}

func (s *State) advanceTurnWithBoard(board *Board, now time.Time) {
	if s.Phase != Playing {
		return
	}
	eliminated := map[Seat]bool{}
	for seat, player := range s.Players {
		eliminated[seat] = player.Eliminated
	}
	next := nextSeat(s.Turn, eliminated)
	if next == "" {
		return
	}
	s.Turn = next
	s.Deadline = now.Add(s.Clock.TurnDuration())
	if len(s.LegalMoves(board, next)) == 0 {
		s.eliminate(next, "no_legal_move")
		if s.Phase != Playing {
			return
		}
		s.advanceTurnWithBoard(board, now)
		return
	}
	if s.teamEliminated(North) {
		s.finish("win", East.Team())
	}
	if s.teamEliminated(East) {
		s.finish("win", North.Team())
	}
}

func (s *State) Tick(board *Board, now time.Time) bool {
	if s.Phase == Setup && !s.SetupDeadline.IsZero() && !now.Before(s.SetupDeadline) {
		for _, seat := range Seats {
			player := s.Players[seat]
			deployment := map[string]Piece{}
			for node, piece := range s.Pieces {
				if piece.Owner == seat {
					deployment[node] = piece
				}
			}
			if player.Username != "" && ValidateDeployment(board, seat, deployment) == nil {
				player.Ready = true
				s.Players[seat] = player
			}
		}
		if err := s.Start(board, now); err == nil {
			return true
		}
		return true
	}
	if s.Phase != Playing || now.Before(s.Deadline) {
		return false
	}
	player := s.Players[s.Turn]
	player.Misses++
	s.Players[s.Turn] = player
	if player.Misses >= 5 {
		s.eliminate(s.Turn, "missed_deadlines")
	}
	s.advanceTurnWithBoard(board, now)
	s.Version++
	return true
}

func (s *State) Resign(board *Board, seat Seat, now time.Time) error {
	if s.Phase != Playing || s.Players[seat].Eliminated {
		return ErrNotPlaying
	}
	s.eliminate(seat, "resigned")
	s.advanceTurnWithBoard(board, now)
	s.Version++
	return nil
}

func (s *State) OfferDraw(seat Seat) error {
	if s.Phase != Playing || s.Players[seat].Eliminated {
		return ErrNotPlaying
	}
	if s.DrawOffer != "" {
		return errors.New("a draw offer is already pending")
	}
	s.DrawOffer = seat
	s.DrawAccepts = map[Seat]bool{seat: true}
	s.Version++
	return nil
}

func (s *State) RespondDraw(seat Seat, accept bool) error {
	if s.DrawOffer == "" || s.Players[seat].Eliminated {
		return errors.New("no draw offer is pending")
	}
	if !accept {
		s.DrawOffer = ""
		s.DrawAccepts = map[Seat]bool{}
		s.Version++
		return nil
	}
	s.DrawAccepts[seat] = true
	for _, active := range s.ActiveSeats() {
		if !s.DrawAccepts[active] {
			s.Version++
			return nil
		}
	}
	s.finish("draw", "unanimous_offer")
	return nil
}

func (s *State) eliminate(seat Seat, reason string) {
	player := s.Players[seat]
	player.Eliminated = true
	player.Ready = false
	player.EliminationReason = reason
	s.Players[seat] = player
	for node, piece := range s.Pieces {
		if piece.Owner == seat {
			delete(s.Pieces, node)
		}
	}
	if s.teamEliminated(seat) {
		s.finish("win", oppositeTeam(seat.Team()))
	}
}

func (s *State) revealFlag(owner Seat) {
	for node, piece := range s.Pieces {
		if piece.Owner == owner && piece.Kind == Flag {
			s.RevealedFlags[owner] = node
			return
		}
	}
}

func oppositeTeam(team string) string {
	if team == "north_south" {
		return "east_west"
	}
	return "north_south"
}

func (s *State) teamEliminated(seat Seat) bool {
	for _, member := range Seats {
		if member.Team() == seat.Team() && !s.Players[member].Eliminated {
			return false
		}
	}
	return true
}

func (s *State) finish(outcome, reason string) {
	if s.Phase == Finished {
		return
	}
	s.Phase = Finished
	s.Deadline = time.Time{}
	s.Result = &Result{Outcome: outcome, Reason: reason}
	if outcome == "win" {
		s.Result.Team = reason
	}
}
