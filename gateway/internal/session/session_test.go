package session

import (
	"testing"
	"time"
)

func TestSessionStateTransition(t *testing.T) {
	s := NewSession(1, "user1", "host1")

	if s.GetState() != StateActive {
		t.Errorf("New session should be active")
	}

	s.SetState(StateDetached)
	if s.GetState() != StateDetached {
		t.Errorf("Session should be detached")
	}

	s.SetState(StateExpired)
	if s.GetState() != StateExpired {
		t.Errorf("Session should be expired")
	}

	s.SetState(StateClosed)
	if s.GetState() != StateClosed {
		t.Errorf("Session should be closed")
	}
}

func TestSessionIsSlow(t *testing.T) {
	s := NewSession(1, "user1", "host1")

	if s.GetSlow() {
		t.Errorf("New session should not be slow")
	}

	s.SetSlow(true)
	if !s.GetSlow() {
		t.Errorf("Session should be slow")
	}

	s.SetSlow(false)
	if s.GetSlow() {
		t.Errorf("Session should not be slow after SetSlow(false)")
	}
}

func TestSessionPromptState(t *testing.T) {
	s := NewSession(1, "user1", "host1")

	if s.GetPromptState() != PromptUnknown {
		t.Errorf("New session should have PromptUnknown")
	}

	s.SetPromptState(PromptShell)
	if s.GetPromptState() != PromptShell {
		t.Errorf("Session prompt state should be PromptShell")
	}

	s.SetPromptState(PromptTUI)
	if s.GetPromptState() != PromptTUI {
		t.Errorf("Session prompt state should be PromptTUI")
	}
}

func TestSessionLastActiveAt(t *testing.T) {
	before := time.Now()
	s := NewSession(1, "user1", "host1")
	after := s.LastActiveAt

	if after.Before(before) || after.Equal(before) {
		t.Errorf("LastActiveAt should be set to approximately current time")
	}
}
