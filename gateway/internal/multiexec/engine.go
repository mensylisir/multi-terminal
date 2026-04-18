package multiexec

import (
	"fmt"
	"sync"

	"github.com/mensylisir/multi-terminal/gateway/internal/session"
)

// Engine handles multi-host input broadcast (Multiexec)
type Engine struct {
	sessionMgr *session.Manager
	mu         sync.Mutex
}

// NewEngine creates a new Multiexec engine
func NewEngine(sessionMgr *session.Manager) *Engine {
	return &Engine{
		sessionMgr: sessionMgr,
	}
}

// Broadcast sends input to all target sessions except the source
// It clones the input and delivers it to each target's shell stream asynchronously
func (e *Engine) Broadcast(input []byte, sourceSessionID uint32, targetSessionIDs []uint32) []error {
	e.mu.Lock()
	defer e.mu.Unlock()

	var errors []error

	for _, targetID := range targetSessionIDs {
		if targetID == sourceSessionID {
			continue // Skip broadcasting to source session
		}

		s, ok := e.sessionMgr.Get(targetID)
		if !ok {
			errors = append(errors, fmt.Errorf("session %d not found", targetID))
			continue
		}

		if s.GetState() != session.StateActive {
			errors = append(errors, fmt.Errorf("session %d not active", targetID))
			continue
		}

		// Check if session is slow - skip if buffer is full
		if s.GetSlow() {
			errors = append(errors, fmt.Errorf("session %d is slow, buffer full", targetID))
			continue
		}

		// Asynchronously write to target session shell stream
		go func(sess *session.Session, data []byte) {
			_, err := sess.ShellStream.Write(data)
			if err != nil {
				// Log error - in production this would go to proper logging
				fmt.Printf("Write error to session %d: %v\n", sess.SessionID, err)
			}
		}(s, input)
	}

	return errors
}

// BroadcastSync sends input to all target sessions synchronously (FIFO ordered)
// Returns errors for any failed sessions
func (e *Engine) BroadcastSync(input []byte, sourceSessionID uint32, targetSessionIDs []uint32) []error {
	e.mu.Lock()
	defer e.mu.Unlock()

	var errors []error

	for _, targetID := range targetSessionIDs {
		if targetID == sourceSessionID {
			continue
		}

		s, ok := e.sessionMgr.Get(targetID)
		if !ok {
			errors = append(errors, fmt.Errorf("session %d not found", targetID))
			continue
		}

		if s.GetState() != session.StateActive {
			errors = append(errors, fmt.Errorf("session %d not active", targetID))
			continue
		}

		if s.GetSlow() {
			errors = append(errors, fmt.Errorf("session %d is slow, buffer full", targetID))
			continue
		}

		// Synchronous write for FIFO ordering
		_, err := s.ShellStream.Write(input)
		if err != nil {
			errors = append(errors, fmt.Errorf("write error to session %d: %v", targetID, err))
		}
	}

	return errors
}

// GetActiveSessions returns all session IDs in active state
func (e *Engine) GetActiveSessions() []uint32 {
	var activeIDs []uint32
	e.sessionMgr.Range(func(s *session.Session) bool {
		if s.GetState() == session.StateActive {
			activeIDs = append(activeIDs, s.SessionID)
		}
		return true
	})
	return activeIDs
}

// GetSlowSessions returns all session IDs marked as slow
func (e *Engine) GetSlowSessions() []uint32 {
	var slowIDs []uint32
	e.sessionMgr.Range(func(s *session.Session) bool {
		if s.GetSlow() {
			slowIDs = append(slowIDs, s.SessionID)
		}
		return true
	})
	return slowIDs
}