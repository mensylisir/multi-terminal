package session

import (
	"fmt"
	"time"
)

const (
	DefaultReconnectTTL = 10 * time.Minute
)

// ReconnectInfo contains information needed to reconnect a session
type ReconnectInfo struct {
	SessionID uint32
	UserID    string
	HostID    string
	Cols      int
	Rows      int
	Offset    int64 // read offset for buffer replay
}

// Detach transitions a session from active to detached state
func (s *Session) Detach() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.State = StateDetached
	s.LastActiveAt = time.Now()
}

// CanReconnect checks if session can be reconnected (detached and within TTL)
func (s *Session) CanReconnect() bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	if s.State != StateDetached {
		return false
	}
	return time.Since(s.LastActiveAt) <= DefaultReconnectTTL
}

// IsExpired checks if the detached session has exceeded TTL
func (s *Session) IsExpired() bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	if s.State != StateDetached {
		return false
	}
	return time.Since(s.LastActiveAt) > DefaultReconnectTTL
}

// GetReconnectInfo returns information needed to reconnect
func (s *Session) GetReconnectInfo() *ReconnectInfo {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return &ReconnectInfo{
		SessionID: s.SessionID,
		UserID:    s.UserID,
		HostID:    s.HostID,
		Cols:      s.Cols,
		Rows:      s.Rows,
		Offset:    0,
	}
}

// Attach transitions a session from detached to active state
func (s *Session) Attach() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.State != StateDetached {
		return fmt.Errorf("session %d is not in detached state", s.SessionID)
	}

	if time.Since(s.LastActiveAt) > DefaultReconnectTTL {
		s.State = StateExpired
		return fmt.Errorf("session %d has expired", s.SessionID)
	}

	s.State = StateActive
	s.LastActiveAt = time.Now()
	return nil
}

// ReconnectHandler handles session reconnection requests
type ReconnectHandler struct {
	manager *Manager
}

// NewReconnectHandler creates a new ReconnectHandler
func NewReconnectHandler(manager *Manager) *ReconnectHandler {
	return &ReconnectHandler{manager: manager}
}

// HandleReconnect attempts to reconnect a detached session
func (h *ReconnectHandler) HandleReconnect(sessionID uint32) (*Session, error) {
	s, ok := h.manager.Get(sessionID)
	if !ok {
		return nil, fmt.Errorf("session not found: %d", sessionID)
	}

	if err := s.Attach(); err != nil {
		return nil, err
	}

	return s, nil
}