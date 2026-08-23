package game

import (
	"errors"
	"time"
)

// BeginSetup moves a full room from the lobby into deployment without marking
// the caller ready. Every player must still arrange and confirm their own army.
func (s *State) BeginSetup(now time.Time) error {
	if s.Phase != Lobby {
		return errors.New("deployment has already started")
	}
	for _, seat := range Seats {
		if s.Players[seat].Username == "" {
			return ErrNotReady
		}
	}
	s.Phase = Setup
	s.SetupDeadline = now.Add(s.Clock.SetupDuration())
	s.Version++
	return nil
}

func (s *State) Pause(now time.Time) error {
	if s.Paused {
		return errors.New("match is already paused")
	}
	if s.Phase != Playing {
		return errors.New("only an active match can be paused")
	}
	s.Paused = true
	s.PausedRemaining = remainingTime(s.Deadline, now)
	s.Deadline = time.Time{}
	s.Version++
	return nil
}

func (s *State) Resume(now time.Time) error {
	if !s.Paused {
		return errors.New("match is not paused")
	}
	if s.Phase != Playing {
		return errors.New("only a paused match can be resumed")
	}
	remaining := s.PausedRemaining
	if remaining <= 0 {
		remaining = s.Clock.TurnDuration()
	}
	s.Deadline = now.Add(remaining)
	s.Paused = false
	s.PausedRemaining = 0
	s.Version++
	return nil
}

func (s *State) Stop() error {
	if s.Phase != Setup && s.Phase != Playing {
		return errors.New("no active match to stop")
	}
	s.Paused = false
	s.PausedRemaining = 0
	s.SetupDeadline = time.Time{}
	s.finish("stopped", "manual_stop")
	s.Version++
	return nil
}

func remainingTime(deadline, now time.Time) time.Duration {
	if deadline.IsZero() {
		return 0
	}
	remaining := deadline.Sub(now)
	if remaining < 0 {
		return 0
	}
	return remaining
}
